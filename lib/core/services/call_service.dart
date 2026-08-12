import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_ringtone_player/flutter_ringtone_player.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import 'package:vibration/vibration.dart';
import 'package:web_socket_channel/io.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

enum CallPhase {
  /// Connected to the signaling room, nothing happening yet.
  idle,

  /// We're the caller and have sent a call-request, waiting for the
  /// other side to accept/reject.
  ringingOutgoing,

  /// We're the callee and received a call-request, waiting for the
  /// local user to accept/reject.
  ringingIncoming,

  /// Accepted (either side) — WebRTC offer/answer/ICE is in progress.
  connecting,

  /// Peer connection established, media flowing.
  active,

  /// Call ended, rejected, or the other side went "busy" — terminal.
  ended,

  /// Signaling connection itself failed.
  failed,
}

/// A minimal peer-to-peer WebRTC video call over the backend's signaling
/// relay (`GET /ws/call/:loanId` — internal/signaling/handler.go). The
/// server only ever relays small JSON control/SDP/ICE messages; once
/// connected, audio/video flows directly between the two devices.
///
/// One instance is a single call attempt for one loan — construct a new
/// one (or call [reset]) per call.
class CallService extends ChangeNotifier {
  CallService({required this.wsBaseUrl});

  /// e.g. "https://nbfc-backend-gr1t.onrender.com" — converted internally
  /// to the matching wss:// signaling URL.
  final String wsBaseUrl;

  CallPhase phase = CallPhase.idle;
  String? errorMessage;
  String? remoteDisplayName;

  /// True once the call ever actually connected (media flowing), even if
  /// it has since ended — distinguishes "we had a real verification
  /// call" from "declined/busy/failed to connect", for whatever the
  /// caller wants to do post-call (celebration screen, moving on to
  /// document capture, ...).
  bool wasEverActive = false;
  bool micMuted = false;
  bool cameraOff = false;
  bool _frontCamera = true;
  bool get isFrontCamera => _frontCamera;

  /// Whether the local camera/mic stream has actually been granted yet —
  /// UI uses this to show a placeholder instead of an empty black box
  /// while the permission prompt/getUserMedia call is still in flight.
  bool get hasLocalPreview => _localStream != null;

  final RTCVideoRenderer localRenderer = RTCVideoRenderer();
  final RTCVideoRenderer remoteRenderer = RTCVideoRenderer();

  WebSocketChannel? _channel;
  RTCPeerConnection? _pc;
  MediaStream? _localStream;
  bool _isCaller = false;
  bool _renderersInitialized = false;
  final _ringtonePlayer = FlutterRingtonePlayer();
  bool _ringing = false;
  final List<RTCIceCandidate> _pendingRemoteCandidates = [];
  Timer? _keepAliveTimer;

  // Public STUN only (no TURN relay configured) — this is a genuine
  // peer-to-peer call as requested: STUN merely helps each device
  // discover its own reachable address for the other side to connect
  // to directly, it never relays media itself. On networks where a
  // direct P2P path isn't reachable (symmetric NAT, some carrier
  // networks) the call may fail to connect without a TURN server, which
  // would need a paid/hosted relay to add.
  static const _iceServers = [
    {'urls': 'stun:stun.l.google.com:19302'},
    {'urls': 'stun:stun1.l.google.com:19302'},
  ];

  Future<void> _initRenderers() async {
    if (_renderersInitialized) return;
    await localRenderer.initialize();
    await remoteRenderer.initialize();
    _renderersInitialized = true;
  }

  /// Opens the signaling connection for [loanId] and starts listening.
  /// [isCaller] determines who initiates the WebRTC offer once a call is
  /// accepted (always the employee in this app's flow).
  Future<void> connect({required String token, required String loanId, required bool isCaller}) async {
    await _initRenderers();
    _isCaller = isCaller;
    final wsUrl = _toWsUrl(wsBaseUrl, loanId);
    try {
      _channel = IOWebSocketChannel.connect(Uri.parse(wsUrl), headers: {'Authorization': 'Bearer $token'});
      _channel!.stream.listen(
        (raw) => _handleSignal(jsonDecode(raw as String) as Map<String, dynamic>),
        onError: (_) => _fail('Connection lost.'),
        onDone: () {
          if (phase != CallPhase.ended && phase != CallPhase.failed) _fail('Connection lost.');
        },
      );
      // A connection that only ever *listens* (the customer's background
      // incoming-call socket never sends anything while idle) can go
      // silently dead — NAT/firewall/mobile-carrier timeouts close idle
      // sockets without either side seeing an error or close event, so
      // the app would sit there looking "connected" forever and simply
      // never receive a future call-request. Sending a small ping
      // periodically both keeps the path alive through those timeout
      // windows and, if the socket really is dead, makes that failure
      // surface immediately (via onError above) instead of silently.
      _keepAliveTimer?.cancel();
      _keepAliveTimer = Timer.periodic(const Duration(seconds: 20), (_) => _send({'type': 'ping'}));
    } catch (_) {
      _fail('Could not reach the call server.');
    }
  }

  static String _toWsUrl(String httpBase, String loanId) {
    final wsBase = httpBase.replaceFirst('https://', 'wss://').replaceFirst('http://', 'ws://');
    return '$wsBase/ws/call/$loanId';
  }

  /// Caller-side: announce the call to the other party once connected.
  void sendCallRequest(String callerName) {
    phase = CallPhase.ringingOutgoing;
    notifyListeners();
    _send({'type': 'call-request', 'name': callerName});
  }

  Future<void> _handleSignal(Map<String, dynamic> data) async {
    switch (data['type']) {
      case 'peer-joined':
        // The callee connecting is the caller's cue that the other side
        // is reachable — nothing to do until a call-request is sent
        // (caller) or received (callee).
        break;
      case 'call-request':
        remoteDisplayName = data['name'] as String?;
        phase = CallPhase.ringingIncoming;
        notifyListeners();
        _startRinging();
        break;
      case 'call-accept':
        if (_isCaller) {
          phase = CallPhase.connecting;
          notifyListeners();
          await _startAsCaller();
        }
        break;
      case 'call-reject':
        _end('The call was declined.');
        break;
      case 'busy':
        _end('The other party is busy right now.');
        break;
      case 'call-end':
        _end('Call ended.');
        break;
      case 'offer':
        await _handleOffer(data);
        break;
      case 'answer':
        await _pc?.setRemoteDescription(RTCSessionDescription(data['sdp'] as String, 'answer'));
        break;
      case 'ice-candidate':
        final cand = RTCIceCandidate(
          data['candidate'] as String?,
          data['sdpMid'] as String?,
          data['sdpMLineIndex'] as int?,
        );
        if (_pc == null) {
          _pendingRemoteCandidates.add(cand);
        } else {
          await _pc!.addCandidate(cand);
        }
        break;
    }
  }

  /// Callee-side: accept an incoming call — grabs local media and waits
  /// for the caller's offer.
  Future<void> accept() async {
    _stopRinging();
    phase = CallPhase.connecting;
    notifyListeners();
    _send({'type': 'call-accept'});
    await ensureLocalStream();
  }

  void reject() {
    _stopRinging();
    _send({'type': 'call-reject'});
    _end(null);
  }

  void sendBusy() {
    _stopRinging();
    _send({'type': 'busy'});
    _end(null);
  }

  Future<void> hangUp() async {
    _send({'type': 'call-end'});
    _end(null);
  }

  Future<void> _startAsCaller() async {
    await ensureLocalStream();
    await _ensurePeerConnection();
    final offer = await _pc!.createOffer({'offerToReceiveAudio': 1, 'offerToReceiveVideo': 1});
    await _pc!.setLocalDescription(offer);
    _send({'type': 'offer', 'sdp': offer.sdp});
  }

  Future<void> _handleOffer(Map<String, dynamic> data) async {
    await _ensurePeerConnection();
    await _pc!.setRemoteDescription(RTCSessionDescription(data['sdp'] as String, 'offer'));
    for (final c in _pendingRemoteCandidates) {
      await _pc!.addCandidate(c);
    }
    _pendingRemoteCandidates.clear();
    final answer = await _pc!.createAnswer({'offerToReceiveAudio': 1, 'offerToReceiveVideo': 1});
    await _pc!.setLocalDescription(answer);
    _send({'type': 'answer', 'sdp': answer.sdp});
  }

  /// Public so CallScreen can request the camera/mic as soon as the call
  /// UI opens — previously local media was only grabbed once a call was
  /// actually accepted/placed, so there was no self-view (and no working
  /// camera-switch button, since there was no video track to switch yet)
  /// during the entire ringing/connecting phase, which is most of what a
  /// user actually sees before a call connects.
  Future<void> ensureLocalStream() async {
    if (_localStream != null) return;
    _localStream = await navigator.mediaDevices.getUserMedia({
      'audio': true,
      'video': {'facingMode': _frontCamera ? 'user' : 'environment'},
    });
    localRenderer.srcObject = _localStream;
    notifyListeners();
  }

  Future<void> _ensurePeerConnection() async {
    if (_pc != null) return;
    await ensureLocalStream();
    _pc = await createPeerConnection({'iceServers': _iceServers});
    for (final track in _localStream!.getTracks()) {
      await _pc!.addTrack(track, _localStream!);
    }
    _pc!.onIceCandidate = (candidate) {
      if (candidate.candidate == null) return;
      _send({
        'type': 'ice-candidate',
        'candidate': candidate.candidate,
        'sdpMid': candidate.sdpMid,
        'sdpMLineIndex': candidate.sdpMLineIndex,
      });
    };
    _pc!.onTrack = (event) {
      if (event.streams.isNotEmpty) {
        remoteRenderer.srcObject = event.streams.first;
        phase = CallPhase.active;
        wasEverActive = true;
        notifyListeners();
      }
    };
    _pc!.onConnectionState = (state) {
      if (state == RTCPeerConnectionState.RTCPeerConnectionStateFailed ||
          state == RTCPeerConnectionState.RTCPeerConnectionStateDisconnected) {
        if (phase == CallPhase.active || phase == CallPhase.connecting) {
          _fail('Call connection lost.');
        }
      }
    };
  }

  void toggleMic() {
    micMuted = !micMuted;
    _localStream?.getAudioTracks().forEach((t) => t.enabled = !micMuted);
    notifyListeners();
  }

  void toggleCamera() {
    cameraOff = !cameraOff;
    _localStream?.getVideoTracks().forEach((t) => t.enabled = !cameraOff);
    notifyListeners();
  }

  Future<void> switchCamera() async {
    final videoTrack = _localStream?.getVideoTracks().firstOrNull;
    if (videoTrack == null) return;
    await Helper.switchCamera(videoTrack);
    _frontCamera = !_frontCamera;
    notifyListeners();
  }

  void _send(Map<String, dynamic> message) {
    // Writing to an already-broken sink (most likely from the periodic
    // keepalive ping firing after the socket died) can throw
    // synchronously — that would otherwise be an uncaught exception
    // inside a Timer callback. The stream's own onError/onDone (set up
    // in connect()) is what actually drives the failure/reconnect path;
    // this just needs to not crash on the way there.
    try {
      _channel?.sink.add(jsonEncode(message));
    } catch (_) {
      _fail('Connection lost.');
    }
  }

  void _end(String? message) {
    errorMessage = message;
    phase = CallPhase.ended;
    notifyListeners();
    _teardownMedia();
  }

  void _fail(String message) {
    errorMessage = message;
    phase = CallPhase.failed;
    notifyListeners();
    _teardownMedia();
  }

  void _teardownMedia() {
    _stopRinging();
    _keepAliveTimer?.cancel();
    _keepAliveTimer = null;
    _pc?.close();
    _pc = null;
    _localStream?.getTracks().forEach((t) => t.stop());
    _localStream?.dispose();
    _localStream = null;
  }

  /// Starts the incoming-call alert: the platform's default ringtone
  /// (looping) plus a repeating vibration pattern. Neither existed at
  /// all before — the incoming-call screen was purely visual, so a
  /// customer not actively looking at their phone at that exact moment
  /// had no way to notice a call was happening.
  void _startRinging() {
    if (_ringing) return;
    _ringing = true;
    _ringtonePlayer.playRingtone(looping: true, volume: 1.0);
    Vibration.hasVibrator().then((has) {
      if (has == true && _ringing) {
        Vibration.vibrate(pattern: [0, 800, 400, 800, 400, 800], repeat: 0);
      }
    });
  }

  void _stopRinging() {
    if (!_ringing) return;
    _ringing = false;
    _ringtonePlayer.stop();
    Vibration.cancel();
  }

  @override
  void dispose() {
    _teardownMedia();
    _channel?.sink.close();
    if (_renderersInitialized) {
      localRenderer.dispose();
      remoteRenderer.dispose();
    }
    super.dispose();
  }
}
