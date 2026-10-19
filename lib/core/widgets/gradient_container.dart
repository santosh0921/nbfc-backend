import 'package:flutter/material.dart';
import '../theme/app_radius.dart';
import 'premium_shine.dart';

class GradientContainer extends StatelessWidget {
  final List<Color> colors;
  final Widget child;
  final double radius;
  final EdgeInsetsGeometry padding;
  final AlignmentGeometry begin;
  final AlignmentGeometry end;
  final List<BoxShadow>? boxShadow;

  /// Adds the premium diagonal light-reflection detail — use for the one
  /// or two hero gradient surfaces per screen that should read as a
  /// featured highlight (e.g. a pre-approved offer banner), not every
  /// gradient container.
  final bool shine;

  const GradientContainer({
    super.key,
    required this.colors,
    required this.child,
    this.radius = AppRadius.lg,
    this.padding = const EdgeInsets.all(20),
    this.begin = Alignment.topLeft,
    this.end = Alignment.bottomRight,
    this.boxShadow,
    this.shine = false,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      clipBehavior: shine ? Clip.antiAlias : Clip.none,
      decoration: BoxDecoration(
        gradient: LinearGradient(colors: colors, begin: begin, end: end),
        borderRadius: BorderRadius.circular(radius),
        boxShadow: boxShadow ??
            [
              BoxShadow(
                color: colors.last.withValues(alpha: 0.28),
                blurRadius: 24,
                offset: const Offset(0, 12),
              ),
            ],
      ),
      child: shine
          ? Stack(
              children: [
                Padding(padding: padding, child: child),
                const Positioned.fill(child: PremiumShine(opacity: 0.3)),
              ],
            )
          : Padding(padding: padding, child: child),
    );
  }
}
