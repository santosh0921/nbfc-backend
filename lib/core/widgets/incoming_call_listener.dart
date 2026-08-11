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

class _IncomingCallListenerState extends State<IncomingCallListener> {
  CallService? _callService;
  bool _showingCall = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _startListening());
  }

  Future<void> _startListening() async {
    final token = AuthProvider.instance.token;
    if (token == null) return;

    List<LoanApplication> loans;
    try {
      loans = await LoanApiService.mine(token);
    } catch (_) {
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

  void _onCallServiceChange() {
    final service = _callService;
    if (service == null || _showingCall) return;
    if (service.phase == CallPhase.ringingIncoming) {
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
    }
  }

  @override
  void dispose() {
    _callService?.removeListener(_onCallServiceChange);
    _callService?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => widget.child;
}
