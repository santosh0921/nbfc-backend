import 'dart:math' as math;
import 'package:flutter/material.dart';

class _Particle {
  _Particle({required this.dx, required this.baseY, required this.radius, required this.speed, required this.phase, required this.opacity});

  final double dx; // horizontal position, 0..1 of width
  final double baseY; // vertical anchor, 0..1 of height
  final double radius;
  final double speed; // drift speed multiplier
  final double phase; // offset so particles don't move in lockstep
  final double opacity;
}

/// Soft floating gold/white particles drifting slowly upward — a subtle
/// ambient premium touch, not a literal confetti effect. Deterministic
/// (seeded) so the layout is stable across rebuilds, animated purely via
/// the [progress] ticker (0..1, looping) passed in by the parent.
class ParticlePainter extends CustomPainter {
  ParticlePainter({required this.progress});

  final double progress;

  static final List<_Particle> _particles = List.generate(18, (i) {
    final rng = math.Random(i * 31 + 7);
    return _Particle(
      dx: rng.nextDouble(),
      baseY: rng.nextDouble(),
      radius: 1.2 + rng.nextDouble() * 2.4,
      speed: 0.4 + rng.nextDouble() * 0.8,
      phase: rng.nextDouble(),
      opacity: 0.12 + rng.nextDouble() * 0.22,
    );
  });

  @override
  void paint(Canvas canvas, Size size) {
    for (final p in _particles) {
      // Each particle drifts upward and loops back to the bottom.
      final t = (progress + p.phase) % 1.0;
      final rawY = (p.baseY * size.height - t * size.height * p.speed) % size.height;
      final y = rawY < 0 ? rawY + size.height : rawY;
      final x = p.dx * size.width + math.sin((t + p.phase) * 2 * math.pi) * 8;

      final paint = Paint()
        ..color = Colors.white.withValues(alpha: p.opacity * (1 - (t - 0.5).abs()))
        ..style = PaintingStyle.fill;
      canvas.drawCircle(Offset(x, y), p.radius, paint);
    }
  }

  @override
  bool shouldRepaint(covariant ParticlePainter oldDelegate) => oldDelegate.progress != progress;
}
