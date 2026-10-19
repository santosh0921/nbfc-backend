import 'package:flutter/material.dart';
import '../theme/app_colors.dart';
import '../theme/app_radius.dart';
import 'press_scale.dart';

enum PremiumButtonVariant { primary, secondary, destructive }

/// The one button used everywhere an important action needs a consistent,
/// tactile, premium feel: gold-filled for [PremiumButtonVariant.primary],
/// bordered neutral for [PremiumButtonVariant.secondary], minimal red for
/// [PremiumButtonVariant.destructive]. Wraps [PressScale] for the tap
/// feedback the design system calls for.
class PremiumButton extends StatelessWidget {
  const PremiumButton({
    super.key,
    required this.label,
    this.onPressed,
    this.variant = PremiumButtonVariant.primary,
    this.icon,
    this.loading = false,
    this.expand = true,
  });

  final String label;
  final VoidCallback? onPressed;
  final PremiumButtonVariant variant;
  final IconData? icon;
  final bool loading;
  final bool expand;

  @override
  Widget build(BuildContext context) {
    final bool isDark = Theme.of(context).brightness == Brightness.dark;
    final bool disabled = onPressed == null || loading;

    final Color background;
    final Color foreground;
    final Border? border;
    final List<BoxShadow> shadow;

    switch (variant) {
      case PremiumButtonVariant.primary:
        background = disabled ? AppColors.primary.withValues(alpha: 0.4) : AppColors.primary;
        foreground = AppColors.onPrimary;
        border = null;
        shadow = disabled
            ? []
            : [BoxShadow(color: AppColors.primaryDark.withValues(alpha: 0.28), blurRadius: 16, offset: const Offset(0, 6))];
        break;
      case PremiumButtonVariant.secondary:
        background = isDark ? AppColors.surfaceDark : Colors.white;
        foreground = isDark ? AppColors.textPrimaryDark : AppColors.textPrimaryLight;
        border = Border.all(color: AppColors.primaryDark.withValues(alpha: 0.6), width: 1.4);
        shadow = const [];
        break;
      case PremiumButtonVariant.destructive:
        background = disabled ? AppColors.error.withValues(alpha: 0.15) : AppColors.error.withValues(alpha: 0.1);
        foreground = AppColors.error;
        border = Border.all(color: AppColors.error.withValues(alpha: 0.4));
        shadow = const [];
        break;
    }

    final Widget content = loading
        ? SizedBox(
            height: 18,
            width: 18,
            child: CircularProgressIndicator(strokeWidth: 2.2, valueColor: AlwaysStoppedAnimation(foreground)),
          )
        : Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (icon != null) ...[Icon(icon, size: 18, color: foreground), const SizedBox(width: 8)],
              Text(label, style: TextStyle(color: foreground, fontWeight: FontWeight.w600, fontSize: 15)),
            ],
          );

    final button = Container(
      width: expand ? double.infinity : null,
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
      decoration: BoxDecoration(
        color: background,
        border: border,
        borderRadius: BorderRadius.circular(AppRadius.sm),
        boxShadow: shadow,
      ),
      alignment: Alignment.center,
      child: content,
    );

    return PressScale(onTap: disabled ? null : onPressed, child: button);
  }
}
