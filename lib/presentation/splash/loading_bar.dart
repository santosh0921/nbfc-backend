import 'package:flutter/material.dart';
import 'animated_background.dart';

/// Custom pill-shaped loading bar: white translucent track, an animated
/// navy → royal-blue → gold gradient fill that grows with [fillProgress]
/// (0..1), and a soft gold shimmer sweeping across the filled portion
/// driven by [shimmerProgress] (0..1, looping).
class LoadingBar extends StatelessWidget {
  const LoadingBar({
    super.key,
    required this.fillProgress,
    required this.shimmerProgress,
    this.width = 180,
    this.height = 6,
  });

  final double fillProgress;
  final double shimmerProgress;
  final double width;
  final double height;

  @override
  Widget build(BuildContext context) {
    final clamped = fillProgress.clamp(0.0, 1.0);
    return RepaintBoundary(
      child: Container(
        width: width,
        height: height,
        decoration: BoxDecoration(
          color: SplashPalette.pureWhite.withValues(alpha: 0.16),
          borderRadius: BorderRadius.circular(height),
        ),
        child: Align(
          alignment: Alignment.centerLeft,
          child: FractionallySizedBox(
            widthFactor: clamped,
            child: ClipRRect(
              borderRadius: BorderRadius.circular(height),
              child: Stack(
                children: [
                  Container(
                    decoration: const BoxDecoration(
                      gradient: LinearGradient(
                        colors: [
                          SplashPalette.royalBlue,
                          SplashPalette.premiumGold,
                          SplashPalette.lightGold,
                        ],
                      ),
                    ),
                  ),
                  if (clamped > 0)
                    Positioned.fill(
                      child: FractionallySizedBox(
                        alignment: Alignment(-1 + shimmerProgress * 3, 0),
                        widthFactor: 0.35,
                        child: DecoratedBox(
                          decoration: BoxDecoration(
                            gradient: LinearGradient(
                              colors: [
                                Colors.white.withValues(alpha: 0),
                                Colors.white.withValues(alpha: 0.55),
                                Colors.white.withValues(alpha: 0),
                              ],
                            ),
                          ),
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
