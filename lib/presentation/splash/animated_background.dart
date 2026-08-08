import 'package:flutter/material.dart';
import 'glow_widget.dart';
import 'particle_painter.dart';

/// Splash-only brand palette for the "Jayashri Capital" launch
/// experience. Deliberately kept local to this feature (not merged into
/// the app's global [AppColors]) — the rest of the app keeps its
/// existing blue/gold theme untouched.
class SplashPalette {
  SplashPalette._();

  static const Color primaryNavy = Color(0xFF071A3D);
  static const Color royalBlue = Color(0xFF0B5FFF);
  static const Color premiumGold = Color(0xFFF8C146);
  static const Color lightGold = Color(0xFFFFD978);
  static const Color pureWhite = Color(0xFFFFFFFF);
  static const Color offWhite = Color(0xFFF8FAFC);
}

/// Layered, slowly-drifting premium background: a navy base gradient,
/// a royal-blue ambient glow, a golden radial glow, two large blurred
/// accent circles, and soft upward-floating particles. Everything is
/// driven off a single ambient [pulse] value (0..1, looping) so the
/// whole layer repaints together under one RepaintBoundary.
class AnimatedBackground extends StatelessWidget {
  const AnimatedBackground({super.key, required this.pulse});

  /// 0..1, looping — drives the slow ambient movement of every layer.
  final double pulse;

  @override
  Widget build(BuildContext context) {
    // A gentle back-and-forth breathing value derived from the looping
    // pulse, so glows/circles ease in both directions rather than snap.
    final breathe = (pulse < 0.5 ? pulse : 1 - pulse) * 2; // 0..1..0

    return RepaintBoundary(
      child: Stack(
        fit: StackFit.expand,
        children: [
          // Base navy → royal-blue diagonal gradient.
          DecoratedBox(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment(-1 + breathe * 0.15, -1),
                end: Alignment(1, 1 - breathe * 0.15),
                colors: const [
                  SplashPalette.primaryNavy,
                  Color(0xFF0A2555),
                  SplashPalette.primaryNavy,
                ],
              ),
            ),
          ),
          // Royal blue ambient glow, upper-left.
          Positioned(
            top: -60 + breathe * 20,
            left: -60,
            child: GlowWidget(pulse: breathe, color: SplashPalette.royalBlue, size: 320, minOpacity: 0.28, maxOpacity: 0.5),
          ),
          // Golden radial glow, lower-right.
          Positioned(
            bottom: -80 + breathe * 24,
            right: -60,
            child: GlowWidget(pulse: 1 - breathe, color: SplashPalette.premiumGold, size: 300, minOpacity: 0.16, maxOpacity: 0.34),
          ),
          // Soft blurred accent circle, center-right.
          Positioned(
            top: 140 - breathe * 16,
            right: -100,
            child: GlowWidget(pulse: breathe, color: SplashPalette.royalBlue, size: 220, minOpacity: 0.1, maxOpacity: 0.22),
          ),
          // Floating particles.
          Positioned.fill(
            child: CustomPaint(painter: ParticlePainter(progress: pulse)),
          ),
        ],
      ),
    );
  }
}
