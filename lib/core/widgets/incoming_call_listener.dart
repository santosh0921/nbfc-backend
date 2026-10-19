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

/// One persistent signaling listener for one call "room" — either the
/// customer's current in-progress loan (employee field-verification calls)
/// or their own `kyc-<customerId>` room (KYC-officer video-verification
/// calls, requested from the KYC portal — see the video-verification-
/// request feature). Two independent instances of this run side by side so
/// either kind of call can reach the customer while the app is open; each
/// owns its own socket/reconnect/lifecycle exactly like the original
/// single-room listener did.
class _RoomListener {
  _RoomListener({required this.peerLabel, required this.roomId});

  final String peerLabel;

  /// Either a loan id or `kyc-<customerId>` — resolved once per
  /// (re)connect attempt via [resolveRoomId], since the active loan can
  /// change between connects but the kyc room id cannot.
  final Future<String?> Function() roomId;

  CallService? callService;
  bool showingCall = false;
  int reconnectAttempt = 0;
  Timer? reconnectTimer;

  Future<void> Function()? _startListening;

  void bind(BuildContext Function() contextGetter, VoidCallback onCallEnded) {
    _startListening = () => _doStartListening(contextGetter, onCallEnded);
  }

  Future<void> start() async => _startListening?.call();

  void ensureConnected() {
    if (showingCall) return; // an active call is already using this connection
    final phase = callService?.phase;
    // See the original single-room listener's comment (preserved
    // verbatim in spirit): only a call actually in progress is trusted as
    // proof the socket is alive — everything else, including "idle", gets
    // a forced fresh reconnect on resume, since the OS can silently kill a
    // backgrounded socket with no error/close event.
    if (phase == CallPhase.ringingIncoming || phase == CallPhase.connecting || phase == CallPhase.active) {
      return;
    }
    reconnectTimer?.cancel();
    reconnectAttempt = 0;
    if (callService != null) {
      callService!.removeListener(_onCallServiceChangeRef!);
      callService!.dispose();
      callService = null;
    }
    start();
  }

  VoidCallback? _onCallServiceChangeRef;

  Future<void> _doStartListening(BuildContext Function() contextGetter, VoidCallback onCallEnded) async {
    final token = AuthProvider.instance.token;
    if (token == null) return;

    final id = await roomId();
    if (id == null) return;

    final service = CallService(wsBaseUrl: ApiClient.baseUrl);
    callService = service;
    void onChange() => _onCallServiceChange(service, token, contextGetter, onCallEnded);
    _onCallServiceChangeRef = onChange;
    service.addListener(onChange);
    try {
      await service.connect(token: token, loanId: id, isCaller: false);
    } catch (_) {
      scheduleReconnect(contextGetter, onCallEnded);
    }
  }

  void scheduleReconnect(BuildContext Function() contextGetter, VoidCallback onCallEnded) {
    reconnectTimer?.cancel();
    final delaySeconds = math.min(30, 2 << reconnectAttempt);
    reconnectAttempt++;
    reconnectTimer = Timer(Duration(seconds: delaySeconds), () {
      if (showingCall) return;
      if (callService != null && _onCallServiceChangeRef != null) {
        callService!.removeListener(_onCallServiceChangeRef!);
      }
      callService?.dispose();
      callService = null;
      start();
    });
  }

  void _onCallServiceChange(
    CallService service,
    String token,
    BuildContext Function() contextGetter,
    VoidCallback onCallEnded,
  ) {
    if (showingCall) return;

    if (service.phase == CallPhase.ringingIncoming) {
      reconnectAttempt = 0;
      showingCall = true;
      final context = contextGetter();
      Navigator.of(context, rootNavigator: true)
          .push(
        MaterialPageRoute(
          builder: (_) => CallScreen(
            callService: service,
            token: token,
            loanId: '', // unused on the callee path — CallScreen skips connect() since phase != idle
            isCaller: false,
            localDisplayName: '',
            peerLabel: peerLabel,
          ),
        ),
      )
          .then((_) async {
        final wasActive = service.wasEverActive;
        if (wasActive) {
          final ctx = contextGetter();
          await Navigator.of(ctx, rootNavigator: true).push(
            MaterialPageRoute(builder: (_) => const CallCompletedCelebrationPage()),
          );
        }
        if (_onCallServiceChangeRef != null) service.removeListener(_onCallServiceChangeRef!);
        service.dispose();
        showingCall = false;
        onCallEnded();
        start();
      });
    } else if (service.phase == CallPhase.failed) {
      scheduleReconnect(contextGetter, onCallEnded);
    }
  }

  void dispose() {
    reconnectTimer?.cancel();
    if (callService != null && _onCallServiceChangeRef != null) {
      callService!.removeListener(_onCallServiceChangeRef!);
    }
    callService?.dispose();
  }
}

class _IncomingCallListenerState extends State<IncomingCallListener> with WidgetsBindingObserver {
  // Loan-verification calls (employee app, field officer) — keyed by the
  // customer's current in-progress loan, same resolution as Track
  // Application: the most recent loan that isn't already disbursed or
  // rejected.
  late final _RoomListener _loanRoom = _RoomListener(
    peerLabel: 'Field Officer',
    roomId: () async {
      final token = AuthProvider.instance.token;
      if (token == null) return null;
      List<LoanApplication> loans;
      try {
        loans = await LoanApiService.mine(token);
      } catch (_) {
        rethrow;
      }
      if (loans.isEmpty) return null;
      final inProgress = loans.where((l) => l.status != 'disbursed' && l.status != 'rejected').toList()
        ..sort((a, b) => b.createdAt.compareTo(a.createdAt));
      if (inProgress.isEmpty) return null;
      return '${inProgress.first.id}';
    },
  );

  // KYC-officer video-verification calls (KYC portal, web) — keyed by this
  // customer's own id, a synthetic room the loan-based listener above
  // never covers since KYC verification can happen before any loan even
  // exists. See lib/features/home/widgets/video_verification_request_page.dart
  // for the request/response half of this feature.
  late final _RoomListener _kycRoom = _RoomListener(
    peerLabel: 'KYC Officer',
    roomId: () async {
      final id = AuthProvider.instance.customerId;
      return id == null ? null : 'kyc-$id';
    },
  );

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _loanRoom.bind(() => context, () {});
    _kycRoom.bind(() => context, () {});
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _loanRoom.start();
      _kycRoom.start();
    });
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // The signaling socket can die silently while the app is backgrounded
    // (mobile OSes routinely suspend/kill sockets to save battery) with
    // neither an error nor a close event ever firing — so coming back to
    // the foreground is exactly when a stale connection needs to be
    // actively checked, not just assumed to still be good.
    if (state == AppLifecycleState.resumed) {
      _loanRoom.ensureConnected();
      _kycRoom.ensureConnected();
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _loanRoom.dispose();
    _kycRoom.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => widget.child;
}
