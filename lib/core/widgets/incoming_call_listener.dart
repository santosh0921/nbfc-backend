import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import '../network/api_client.dart';
import '../network/loan_api_service.dart';
import '../providers/auth_provider.dart';
import '../services/call_service.dart';
import '../../models/loan_application.dart';
import '../../features/home/widgets/call_completed_celebration_page.dart';
import 'call_screen.dart';

/// Mounted once at the customer app's shell (present across every tab for
/// the whole logged-in session) — keeps a lightweight signaling
/// connection open for the customer's current in-progress loan so an
/// employee-initiated video call can actually reach them while the app
/// is open. This is the realistic scope without push-notification
/// infrastructure: a call only rings if the customer's app is running
/// (any screen), not if it's fully closed — the same limitation the
/// signaling relay itself has, since there's nothing to wake the app.
class IncomingCallListener extends StatefulWidget {
  const IncomingCallListener({super.key, required this.child});

  final Widget child;

  @override
  State<IncomingCallListener> createState() => _IncomingCallListenerState();
}

class _IncomingCallListenerState extends State<IncomingCallListener> with WidgetsBindingObserver {
  CallService? _callService;
  bool _showingCall = false;
  int _reconnectAttempt = 0;
  Timer? _reconnectTimer;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) => _startListening());
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // The signaling socket can die silently while the app is backgrounded
    // (mobile OSes routinely suspend/kill sockets to save battery) with
    // neither an error nor a close event ever firing — so coming back to
    // the foreground is exactly when a stale connection needs to be
    // actively checked, not just assumed to still be good.
    if (state == AppLifecycleState.resumed) _ensureConnected();
  }

  void _ensureConnected() {
    if (_showingCall) return; // an active call is already using this connection
    final phase = _callService?.phase;
    if (_callService == null || phase == CallPhase.failed || phase == CallPhase.ended) {
      _reconnectTimer?.cancel();
      _reconnectAttempt = 0;
      _startListening();
    }
  }

  Future<void> _startListening() async {
    final token = AuthProvider.instance.token;
    if (token == null) return;

    List<LoanApplication> loans;
    try {
      loans = await LoanApiService.mine(token);
    } catch (_) {
      _scheduleReconnect();
      return;
    }
    if (loans.isEmpty) return;

    // Same "current active loan" resolution as Track Application — the
    // most recent loan that isn't already disbursed or rejected, since
    // video KYC calls happen during verification, before either of
    // those terminal states.
    final inProgress = loans.where((l) => l.status != 'disbursed' && l.status != 'rejected').toList()
      ..sort((a, b) => b.createdAt.compareTo(a.createdAt));
    if (inProgress.isEmpty) return;
    final loanId = '${inProgress.first.id}';

    final service = CallService(wsBaseUrl: ApiClient.baseUrl);
    _callService = service;
    service.addListener(_onCallServiceChange);
    await service.connect(token: token, loanId: loanId, isCaller: false);
  }

  /// Reconnects with a capped exponential backoff (2s, 4s, 8s, ... up to
  /// 30s) rather than either hammering the server in a tight loop or
  /// giving up outright — a real network blip should recover in a few
  /// seconds, while a genuine outage shouldn't be retried every second
  /// for the rest of the session.
  void _scheduleReconnect() {
    _reconnectTimer?.cancel();
    final delaySeconds = math.min(30, 2 << _reconnectAttempt);
    _reconnectAttempt++;
    _reconnectTimer = Timer(Duration(seconds: delaySeconds), () {
      if (!mounted || _showingCall) return;
      _callService?.removeListener(_onCallServiceChange);
      _callService?.dispose();
      _callService = null;
      _startListening();
    });
  }

  void _onCallServiceChange() {
    final service = _callService;
    if (service == null || _showingCall) return;

    if (service.phase == CallPhase.ringingIncoming) {
      _reconnectAttempt = 0;
      _showingCall = true;
      final token = AuthProvider.instance.token;
      if (token == null) return;
      Navigator.of(context, rootNavigator: true)
          .push(
        MaterialPageRoute(
          builder: (_) => CallScreen(
            callService: service,
            token: token,
            loanId: '', // unused on the callee path — CallScreen skips connect() since phase != idle
            isCaller: false,
            localDisplayName: '',
            peerLabel: 'Field Officer',
          ),
        ),
      )
          .then((_) async {
        final wasActive = service.wasEverActive;
        if (wasActive && mounted) {
          await Navigator.of(context, rootNavigator: true).push(
            MaterialPageRoute(builder: (_) => const CallCompletedCelebrationPage()),
          );
        }
        // The call ended (or was declined) — that CallService instance
        // is done; open a fresh listening connection so a later call
        // can still come through.
        service.removeListener(_onCallServiceChange);
        service.dispose();
        _showingCall = false;
        _startListening();
      });
    } else if (service.phase == CallPhase.failed) {
      // The signaling connection itself died (network drop, server
      // restart, ...) — without this, a customer who never happens to
      // relaunch the app would simply never be reachable again for the
      // rest of this session, no matter how many times an employee tries
      // to call.
      _scheduleReconnect();
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _reconnectTimer?.cancel();
    _callService?.removeListener(_onCallServiceChange);
    _callService?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => widget.child;
}
