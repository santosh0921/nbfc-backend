import 'dart:math' as math;
import 'dart:ui';
import 'package:flutter/material.dart';
import 'animated_background.dart';
import 'glow_widget.dart';

/// The premium glassmorphism logo card: frosted glass fill, a golden
/// border, a soft blue drop shadow, and a golden glow breathing behind
/// it. [reveal] drives the one-time entrance (fade + scale + slight
/// rotation); [breathe] drives the continuous ambient float/glow after
/// that, looping for as long as the splash is on screen.
///
/// The icon is [Icons.account_balance_rounded] as a placeholder — swap
/// [icon] (or replace the `Icon(...)` below with an `Image.asset`) once
/// a real Jayashri Capital SVG mark is available; nothing else in this
/// widget needs to change.
class GlassLogo extends StatelessWidget {
  const GlassLogo({
    super.key,
    required this.reveal,
    required this.breathe,
    this.icon = Icons.account_balance_rounded,
    this.size = 140,
  });

  final double reveal;
  final double breathe;
  final IconData icon;
  final double size;

  @override
  Widget build(BuildContext context) {
    final revealScale = Curves.easeOutBack.transform(reveal.clamp(0.0, 1.0));
    final floatOffset = math.sin(breathe * 2 * math.pi) * 6;
    final rotation = math.sin(breathe * 2 * math.pi) * (3 * math.pi / 180); // ±3 degrees

    return Opacity(
      opacity: reveal.clamp(0.0, 1.0),
      child: Transform.translate(
        offset: Offset(0, floatOffset),
        child: Transform.rotate(
          angle: rotation,
          child: Transform.scale(
            scale: 0.7 + 0.3 * revealScale,
            child: RepaintBoundary(
              child: Stack(
                alignment: Alignment.center,
                children: [
                  // Golden glow behind the card, pulsing gently.
                  GlowWidget(
                    pulse: (math.sin(breathe * 2 * math.pi) + 1) / 2,
                    color: SplashPalette.premiumGold,
                    size: size * 1.7,
                    minOpacity: 0.22,
                    maxOpacity: 0.4,
                  ),
                  // Frosted glass card.
                  ClipRRect(
                    borderRadius: BorderRadius.circular(38),
                    child: BackdropFilter(
                      filter: ImageFilter.blur(sigmaX: 18, sigmaY: 18),
                      child: Container(
                        width: size,
                        height: size,
                        decoration: BoxDecoration(
                          borderRadius: BorderRadius.circular(38),
                          gradient: LinearGradient(
                            begin: Alignment.topLeft,
                            end: Alignment.bottomRight,
                            colors: [
                              SplashPalette.pureWhite.withValues(alpha: 0.22),
                              SplashPalette.pureWhite.withValues(alpha: 0.06),
                            ],
                          ),
                          border: Border.all(
                            color: SplashPalette.premiumGold.withValues(alpha: 0.55),
                            width: 1.4,
                          ),
                          boxShadow: [
                            BoxShadow(
                              color: SplashPalette.royalBlue.withValues(alpha: 0.35),
                              blurRadius: 30,
                              offset: const Offset(0, 14),
                            ),
                          ],
                        ),
                        child: Icon(icon, color: SplashPalette.pureWhite, size: size * 0.42),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
