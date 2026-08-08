import 'package:flutter/material.dart';

/// A soft, pulsing radial glow — used behind the logo card and as an
/// ambient background accent. Drives its own breathing animation off an
/// externally-supplied [pulse] value (0..1) so multiple glows can share
/// one ambient ticker instead of each owning a controller.
class GlowWidget extends StatelessWidget {
  const GlowWidget({
    super.key,
    required this.pulse,
    required this.color,
    this.size = 220,
    this.minOpacity = 0.25,
    this.maxOpacity = 0.55,
  });

  final double pulse;
  final Color color;
  final double size;
  final double minOpacity;
  final double maxOpacity;

  @override
  Widget build(BuildContext context) {
    final opacity = minOpacity + (maxOpacity - minOpacity) * pulse;
    final scale = 0.92 + 0.16 * pulse;
    return IgnorePointer(
      child: Transform.scale(
        scale: scale,
        child: Container(
          width: size,
          height: size,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            gradient: RadialGradient(
              colors: [
                color.withValues(alpha: opacity),
                color.withValues(alpha: 0),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
