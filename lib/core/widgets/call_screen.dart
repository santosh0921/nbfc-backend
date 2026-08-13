import 'package:flutter/material.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import '../services/call_service.dart';

/// Full-screen video call UI, shared by the customer and employee apps.
/// Adapts entirely to [CallService.phase] — same widget for the outgoing
/// "Calling…" screen, the incoming ringing screen, and the active in-call
/// screen, so the transition between them is seamless instead of
/// navigating between separate pages mid-call.
class CallScreen extends StatefulWidget {
  const CallScreen({
    super.key,
    required this.callService,
    required this.token,
    required this.loanId,
    required this.isCaller,
    required this.localDisplayName,
    this.peerLabel = 'Customer',
  });

  final CallService callService;
  final String token;
  final String loanId;
  final bool isCaller;
  final String localDisplayName;

  /// What to call the other party before their name is known (e.g. while
  /// still connecting) — "Customer" on the employee side, "Field Officer"
  /// on the customer side.
  final String peerLabel;

  @override
  State<CallScreen> createState() => _CallScreenState();
}

class _CallScreenState extends State<CallScreen> {
  bool _speakerOn = true;

  @override
  void initState() {
    super.initState();
    widget.callService.addListener(_onChange);
    // Grab the camera/mic as soon as this screen opens — for both the
    // caller and the callee — rather than only once a call is actually
    // accepted. Without this there was no self-view (and the
    // camera-switch button had nothing to switch) for the entire
    // ringing/connecting phase, which is most of what a user sees before
    // a call connects; requesting it here also means accept() doesn't
    // have to wait on a fresh getUserMedia/permission round-trip.
    widget.callService.ensureLocalStream();
    // A CallService handed in already past `idle` (e.g. the customer
    // app's incoming-call listener, which stays connected in the
    // background so it can catch a call-request at all) is already
    // live — connecting again here would open a second, redundant
    // signaling socket.
    if (widget.callService.phase == CallPhase.idle) {
      _start();
    }
  }

  Future<void> _start() async {
    await widget.callService.connect(token: widget.token, loanId: widget.loanId, isCaller: widget.isCaller);
    if (widget.isCaller && mounted) {
      widget.callService.sendCallRequest(widget.localDisplayName);
    }
  }

  void _onChange() {
    if (mounted) setState(() {});
    if (widget.callService.phase == CallPhase.active) {
      Helper.setSpeakerphoneOn(_speakerOn);
    }
  }

  @override
  void dispose() {
    widget.callService.removeListener(_onChange);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final call = widget.callService;
    return PopScope(
      canPop: call.phase == CallPhase.ended || call.phase == CallPhase.failed || call.phase == CallPhase.idle,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) call.hangUp();
      },
      child: Scaffold(
        backgroundColor: Colors.black,
        body: SafeArea(
          child: switch (call.phase) {
            CallPhase.idle || CallPhase.ringingOutgoing => _RingingView(call: call, outgoing: true, label: widget.peerLabel),
            CallPhase.ringingIncoming => _RingingView(call: call, outgoing: false, label: widget.peerLabel),
            CallPhase.connecting => _ConnectingView(call: call),
            CallPhase.active => _ActiveCallView(
                call: call,
                speakerOn: _speakerOn,
                onToggleSpeaker: () => setState(() {
                  _speakerOn = !_speakerOn;
                  Helper.setSpeakerphoneOn(_speakerOn);
                }),
              ),
            CallPhase.ended || CallPhase.failed => _EndedView(call: call),
          },
        ),
      ),
    );
  }
}

/// Dark backdrop gradient used behind every pre-connect phase (ringing,
/// connecting) — WhatsApp/typical dialer apps never show a flat black
/// screen for these states, a subtle top-to-bottom gradient reads as much
/// more "designed" than plain black.
class _CallBackdrop extends StatelessWidget {
  const _CallBackdrop({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: const BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
          colors: [Color(0xFF1F2A44), Color(0xFF0B0F1A)],
        ),
      ),
      child: child,
    );
  }
}

/// A slow pulsing ring behind the avatar while ringing — the one visual
/// cue that's genuinely missing from a static avatar+text screen and is
/// present in essentially every phone/video-call UI (WhatsApp included)
/// to communicate "this is actively happening right now", not a frozen
/// screen.
class _PulsingRing extends StatefulWidget {
  const _PulsingRing({required this.child, required this.color});

  final Widget child;
  final Color color;

  @override
  State<_PulsingRing> createState() => _PulsingRingState();
}

class _PulsingRingState extends State<_PulsingRing> with SingleTickerProviderStateMixin {
  late final AnimationController _controller =
      AnimationController(vsync: this, duration: const Duration(milliseconds: 1600))..repeat();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _controller,
      builder: (context, _) {
        return Stack(
          alignment: Alignment.center,
          children: [
            for (final delay in [0.0, 0.5])
              Builder(builder: (context) {
                final t = (_controller.value + delay) % 1.0;
                return Opacity(
                  opacity: (1 - t) * 0.35,
                  child: Transform.scale(
                    scale: 1 + t * 0.6,
                    child: Container(
                      width: 128,
                      height: 128,
                      decoration: BoxDecoration(shape: BoxShape.circle, border: Border.all(color: widget.color, width: 2)),
                    ),
                  ),
                );
              }),
            widget.child,
          ],
        );
      },
    );
  }
}

/// The local camera preview, shown as a small floating box in the same
/// position across every phase (ringing, connecting, active) so it never
/// jumps around as the call progresses. Previously this only existed
/// during an active call — for the entire ringing/connecting phase there
/// was no self-view at all, and the "front/back camera" switch button
/// only appeared once the call was already connected, by which point
/// most of the "is my camera even working" uncertainty had already
/// happened. Rebuilds on every [CallService] change (via the parent's
/// `_onChange` -> setState), which is how it picks up the local stream
/// the moment `ensureLocalStream()` resolves and how the mirror flips
/// immediately after a camera switch.
class _SelfPreview extends StatelessWidget {
  const _SelfPreview({required this.call});

  final CallService call;

  @override
  Widget build(BuildContext context) {
    return Positioned(
      top: 16,
      right: 16,
      width: 100,
      height: 136,
      child: Stack(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(14),
            child: Container(
              width: 100,
              height: 136,
              color: Colors.black54,
              decoration: BoxDecoration(border: Border.all(color: Colors.white24, width: 1)),
              child: call.hasLocalPreview
                  ? RTCVideoView(call.localRenderer, mirror: call.isFrontCamera, objectFit: RTCVideoViewObjectFit.RTCVideoViewObjectFitCover)
                  : const Center(child: Icon(Icons.person_outline, color: Colors.white38, size: 36)),
            ),
          ),
          Positioned(
            right: 4,
            bottom: 4,
            child: Material(
              color: Colors.black45,
              shape: const CircleBorder(),
              child: InkWell(
                customBorder: const CircleBorder(),
                onTap: call.switchCamera,
                child: const Padding(
                  padding: EdgeInsets.all(6),
                  child: Icon(Icons.cameraswitch, color: Colors.white, size: 18),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _RingingView extends StatelessWidget {
  const _RingingView({required this.call, required this.outgoing, required this.label});

  final CallService call;
  final bool outgoing;
  final String label;

  @override
  Widget build(BuildContext context) {
    final name = call.remoteDisplayName ?? label;
    return _CallBackdrop(
      child: Stack(
        children: [
          Column(
            children: [
              const Spacer(flex: 2),
              _PulsingRing(
                color: outgoing ? Colors.white38 : Colors.greenAccent,
                child: CircleAvatar(
                  radius: 56,
                  backgroundColor: Colors.white12,
                  child: Text(name.isNotEmpty ? name[0].toUpperCase() : '?', style: const TextStyle(color: Colors.white, fontSize: 40)),
                ),
              ),
              const SizedBox(height: 22),
              Text(name, style: const TextStyle(color: Colors.white, fontSize: 24, fontWeight: FontWeight.w700)),
              const SizedBox(height: 8),
              Text(
                outgoing ? 'Video calling…' : 'Incoming video call',
                style: const TextStyle(color: Colors.white60, fontSize: 15),
              ),
              const Spacer(flex: 3),
              if (outgoing)
                Padding(
                  padding: const EdgeInsets.only(bottom: 48),
                  child: _CallButton(icon: Icons.call_end, color: Colors.red, onPressed: call.hangUp, label: 'Cancel'),
                )
              else
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 36, vertical: 24),
                  child: Column(
                    children: [
                      // Spread wide (spaceBetween, not centered together)
                      // so decline/accept read as clearly opposite actions
                      // at a glance — the pattern every mobile dialer
                      // (WhatsApp included) uses for incoming calls,
                      // rather than two adjacent buttons that are easy to
                      // mis-tap under the other.
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          _CallButton(icon: Icons.call_end, color: Colors.red, onPressed: call.reject, label: 'Decline'),
                          _CallButton(icon: Icons.videocam, color: Colors.green, onPressed: call.accept, label: 'Accept'),
                        ],
                      ),
                      const SizedBox(height: 20),
                      TextButton(
                        onPressed: call.sendBusy,
                        child: const Text("I'm busy, call you later", style: TextStyle(color: Colors.white60)),
                      ),
                    ],
                  ),
                ),
            ],
          ),
          _SelfPreview(call: call),
        ],
      ),
    );
  }
}

class _ConnectingView extends StatelessWidget {
  const _ConnectingView({required this.call});

  final CallService call;

  @override
  Widget build(BuildContext context) {
    return _CallBackdrop(
      child: Stack(
        children: [
          const Center(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                CircularProgressIndicator(color: Colors.white),
                SizedBox(height: 16),
                Text('Connecting…', style: TextStyle(color: Colors.white70)),
              ],
            ),
          ),
          _SelfPreview(call: call),
        ],
      ),
    );
  }
}

class _ActiveCallView extends StatelessWidget {
  const _ActiveCallView({required this.call, required this.speakerOn, required this.onToggleSpeaker});

  final CallService call;
  final bool speakerOn;
  final VoidCallback onToggleSpeaker;

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        RTCVideoView(call.remoteRenderer, objectFit: RTCVideoViewObjectFit.RTCVideoViewObjectFitCover),
        _SelfPreview(call: call),
        // A subtle bottom gradient behind the control tray keeps the
        // buttons legible over a bright remote video frame, instead of
        // floating raw translucent circles directly on top of it.
        Positioned(
          left: 0,
          right: 0,
          bottom: 0,
          height: 160,
          child: IgnorePointer(
            child: DecoratedBox(
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.bottomCenter,
                  end: Alignment.topCenter,
                  colors: [Colors.black.withValues(alpha: 0.55), Colors.transparent],
                ),
              ),
            ),
          ),
        ),
        Positioned(
          left: 16,
          right: 16,
          bottom: 28,
          // Wrap (not Row) guarantees the control bar never overflows —
          // on a narrow phone 5 circular buttons plus their gaps could
          // exceed the available width and overflow off-screen (visually
          // this showed up as buttons clipped or shoved past the screen
          // edge). Wrap simply drops onto a second line instead.
          child: Wrap(
            alignment: WrapAlignment.center,
            spacing: 16,
            runSpacing: 14,
            children: [
              _CallButton(icon: call.micMuted ? Icons.mic_off : Icons.mic, color: Colors.white24, onPressed: call.toggleMic, small: true),
              _CallButton(
                icon: call.cameraOff ? Icons.videocam_off : Icons.videocam,
                color: Colors.white24,
                onPressed: call.toggleCamera,
                small: true,
              ),
              _CallButton(icon: Icons.cameraswitch, color: Colors.white24, onPressed: call.switchCamera, small: true),
              _CallButton(
                icon: speakerOn ? Icons.volume_up : Icons.hearing,
                color: Colors.white24,
                onPressed: onToggleSpeaker,
                small: true,
              ),
              _CallButton(icon: Icons.call_end, color: Colors.red, onPressed: call.hangUp, small: true),
            ],
          ),
        ),
      ],
    );
  }
}

class _EndedView extends StatelessWidget {
  const _EndedView({required this.call});

  final CallService call;

  @override
  Widget build(BuildContext context) {
    return _CallBackdrop(
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.call_end, color: Colors.white38, size: 48),
            const SizedBox(height: 16),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 32),
              child: Text(
                call.errorMessage ?? 'Call ended',
                textAlign: TextAlign.center,
                style: const TextStyle(color: Colors.white70, fontSize: 16),
              ),
            ),
            const SizedBox(height: 24),
            TextButton(
              onPressed: () => Navigator.of(context).maybePop(),
              child: const Text('Close', style: TextStyle(color: Colors.white)),
            ),
          ],
        ),
      ),
    );
  }
}

class _CallButton extends StatelessWidget {
  const _CallButton({required this.icon, required this.color, required this.onPressed, this.label, this.small = false});

  final IconData icon;
  final Color color;
  final VoidCallback onPressed;
  final String? label;
  final bool small;

  @override
  Widget build(BuildContext context) {
    final size = small ? 52.0 : 64.0;
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Material(
          color: color,
          shape: const CircleBorder(),
          elevation: 3,
          child: InkWell(
            customBorder: const CircleBorder(),
            onTap: onPressed,
            child: SizedBox(width: size, height: size, child: Icon(icon, color: Colors.white, size: small ? 24 : 28)),
          ),
        ),
        if (label != null) ...[
          const SizedBox(height: 8),
          Text(label!, style: const TextStyle(color: Colors.white70, fontSize: 13)),
        ],
      ],
    );
  }
}
