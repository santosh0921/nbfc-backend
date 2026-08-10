import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'animated_background.dart';
import 'glow_widget.dart';

/// The premium logo card: the OneFinance mark, a golden glow breathing
/// behind it, and a soft blue drop shadow. [reveal] drives the one-time
/// entrance (fade + scale + slight rotation); [breathe] drives the
/// continuous ambient float/glow after that, looping for as long as the
/// splash is on screen.
class GlassLogo extends StatelessWidget {
  const GlassLogo({
    super.key,
    required this.reveal,
    required this.breathe,
    this.size = 140,
  });

  final double reveal;
  final double breathe;
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
                  // The logo mark itself — it already carries its own
                  // rounded-square navy/gold card styling, so it's shown
                  // directly rather than nested inside another frosted
                  // glass container.
                  Container(
                    width: size,
                    height: size,
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(38),
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
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(36.6),
                      child: Image.asset('assets/images/logo.png', fit: BoxFit.cover),
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
