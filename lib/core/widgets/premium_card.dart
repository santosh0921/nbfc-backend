import 'package:flutter/material.dart';
import '../theme/app_colors.dart';
import '../theme/app_radius.dart';
import '../theme/app_shadows.dart';
import 'premium_shine.dart';

/// Standard elevated surface card used across the app: white/dark surface,
/// soft shadow, rounded corners, hairline border. Set [shine] on cards
/// that should carry the premium diagonal light-reflection detail —
/// leave it off by default so ordinary list/content cards stay plain per
/// the "don't make every card glass" rule.
class PremiumCard extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry padding;
  final VoidCallback? onTap;
  final double radius;
  final bool shine;

  const PremiumCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(16),
    this.onTap,
    this.radius = AppRadius.lg,
    this.shine = false,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final content = Container(
      clipBehavior: shine ? Clip.antiAlias : Clip.none,
      decoration: BoxDecoration(
        color: isDark ? AppColors.surfaceDark : AppColors.surfaceLight,
        borderRadius: BorderRadius.circular(radius),
        border: Border.all(color: isDark ? AppColors.borderDark : AppColors.borderLight),
        boxShadow: AppShadows.card(isDark: isDark),
      ),
      child: Stack(
        children: [
          Padding(padding: padding, child: child),
          if (shine) const Positioned.fill(child: PremiumShine()),
        ],
      ),
    );

    if (onTap == null) return content;

    return Material(
      color: Colors.transparent,
      borderRadius: BorderRadius.circular(radius),
      child: InkWell(
        borderRadius: BorderRadius.circular(radius),
        onTap: onTap,
        child: content,
      ),
    );
  }
}
