import 'package:flutter/material.dart';
import '../theme/app_colors.dart';
import '../theme/app_radius.dart';
import 'premium_shine.dart';

/// Premium highlight surface: a soft gold-tinted gradient, a thin luminous
/// gold-toned border, and a diagonal shine — reserved for the handful of
/// surfaces per screen that should draw the eye (a hero stat, a featured
/// offer, the current plan). Everyday content should use [PremiumCard]
/// instead; using this everywhere would flatten the hierarchy it exists to
/// create and read as "cheap glassmorphism" rather than a deliberate
/// accent.
///
/// Deliberately no [BackdropFilter] blur — see [PremiumShine]'s doc
/// comment on why a real blur-per-card is a performance trap. The "glass"
/// read comes from the translucent gradient + luminous border + shine
/// instead, which is nearly as convincing and orders of magnitude cheaper.
class GlassCard extends StatelessWidget {
  const GlassCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(20),
    this.onTap,
    this.radius = AppRadius.lg,
  });

  final Widget child;
  final EdgeInsetsGeometry padding;
  final VoidCallback? onTap;
  final double radius;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    final content = Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(radius),
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: isDark
              ? [AppColors.surfaceDark, AppColors.surfaceDark.withValues(alpha: 0.85)]
              : [Colors.white, AppColors.primaryLight.withValues(alpha: 0.35)],
        ),
        border: Border.all(
          color: AppColors.primary.withValues(alpha: isDark ? 0.35 : 0.5),
          width: 1,
        ),
        boxShadow: [
          BoxShadow(
            color: AppColors.primaryDark.withValues(alpha: isDark ? 0.18 : 0.12),
            blurRadius: 24,
            offset: const Offset(0, 10),
          ),
        ],
      ),
      child: Stack(
        children: [
          Padding(padding: padding, child: child),
          const Positioned.fill(child: PremiumShine(opacity: 0.4, goldTint: true)),
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
