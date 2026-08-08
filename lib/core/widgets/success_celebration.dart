import 'dart:math' as math;
import 'package:flutter/material.dart';
import '../theme/app_colors.dart';

/// A premium payment/application-success moment: a checkmark that
/// scales in with a soft glow pulse behind it, plus a small burst of
/// confetti dots — the kind of "wow" flourish top fintech apps use for
/// a successful payment or submission, not just a static green tick.
class SuccessCelebration extends StatefulWidget {
  const SuccessCelebration({super.key, this.size = 108});

  final double size;

  @override
  State<SuccessCelebration> createState() => _SuccessCelebrationState();
}

class _SuccessCelebrationState extends State<SuccessCelebration> with SingleTickerProviderStateMixin {
  late final AnimationController _controller = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1400),
  )..forward();

  late final Animation<double> _checkScale = CurvedAnimation(
    parent: _controller,
    curve: const Interval(0.0, 0.45, curve: Curves.elasticOut),
  );
  late final Animation<double> _glowPulse = CurvedAnimation(
    parent: _controller,
    curve: const Interval(0.0, 0.6, curve: Curves.easeOut),
  );
  late final Animation<double> _confetti = CurvedAnimation(
    parent: _controller,
    curve: const Interval(0.15, 1.0, curve: Curves.easeOut),
  );

  static final List<_ConfettiSpec> _specs = List.generate(14, (i) {
    final rng = math.Random(i * 17);
    return _ConfettiSpec(
      angle: rng.nextDouble() * 2 * math.pi,
      distance: 60 + rng.nextDouble() * 40,
      color: [AppColors.primary, AppColors.secondary, AppColors.success, AppColors.warning][i % 4],
      size: 4 + rng.nextDouble() * 4,
    );
  });

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final boxSize = widget.size * 2.2;
    return SizedBox(
      width: boxSize,
      height: boxSize,
      child: AnimatedBuilder(
        animation: _controller,
        builder: (context, _) {
          return Stack(
            alignment: Alignment.center,
            children: [
              // Confetti burst.
              CustomPaint(
                size: Size(boxSize, boxSize),
                painter: _ConfettiPainter(progress: _confetti.value, specs: _specs),
              ),
              // Soft glow pulse ring.
              Transform.scale(
                scale: 1 + _glowPulse.value * 0.6,
                child: Opacity(
                  opacity: (1 - _glowPulse.value).clamp(0.0, 0.5),
                  child: Container(
                    width: widget.size,
                    height: widget.size,
                    decoration: BoxDecoration(color: AppColors.success.withValues(alpha: 0.3), shape: BoxShape.circle),
                  ),
                ),
              ),
              // Checkmark badge.
              Transform.scale(
                scale: _checkScale.value.clamp(0.0, 1.2),
                child: Container(
                  width: widget.size,
                  height: widget.size,
                  decoration: BoxDecoration(
                    color: AppColors.success.withValues(alpha: 0.12),
                    shape: BoxShape.circle,
                  ),
                  child: Icon(Icons.check_circle_rounded, color: AppColors.success, size: widget.size * 0.56),
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _ConfettiSpec {
  const _ConfettiSpec({required this.angle, required this.distance, required this.color, required this.size});

  final double angle;
  final double distance;
  final Color color;
  final double size;
}

class _ConfettiPainter extends CustomPainter {
  _ConfettiPainter({required this.progress, required this.specs});

  final double progress;
  final List<_ConfettiSpec> specs;

  @override
  void paint(Canvas canvas, Size size) {
    if (progress <= 0) return;
    final center = Offset(size.width / 2, size.height / 2);
    final fade = (1 - progress).clamp(0.0, 1.0);

    for (final spec in specs) {
      final travelled = spec.distance * Curves.easeOut.transform(progress);
      final pos = center + Offset(math.cos(spec.angle), math.sin(spec.angle)) * travelled;
      final paint = Paint()..color = spec.color.withValues(alpha: fade);
      canvas.drawCircle(pos, spec.size * (1 - progress * 0.3), paint);
    }
  }

  @override
  bool shouldRepaint(covariant _ConfettiPainter oldDelegate) => oldDelegate.progress != progress;
}
