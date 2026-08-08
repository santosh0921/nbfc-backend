import 'package:flutter/material.dart';
import '../theme/app_radius.dart';

class GradientContainer extends StatelessWidget {
  final List<Color> colors;
  final Widget child;
  final double radius;
  final EdgeInsetsGeometry padding;
  final AlignmentGeometry begin;
  final AlignmentGeometry end;
  final List<BoxShadow>? boxShadow;

  const GradientContainer({
    super.key,
    required this.colors,
    required this.child,
    this.radius = AppRadius.lg,
    this.padding = const EdgeInsets.all(20),
    this.begin = Alignment.topLeft,
    this.end = Alignment.bottomRight,
    this.boxShadow,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: padding,
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
      child: child,
    );
  }
}
