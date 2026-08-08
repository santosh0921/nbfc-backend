import 'package:flutter/material.dart';
import '../constants/app_colors.dart';
import '../constants/app_spacing.dart';

enum AppButtonVariant { primary, secondary, outline, text, danger }

enum AppButtonSize { small, medium, large }

class AppButton extends StatelessWidget {
  const AppButton({
    super.key,
    required this.label,
    this.onPressed,
    this.variant = AppButtonVariant.primary,
    this.size = AppButtonSize.medium,
    this.icon,
    this.isLoading = false,
    this.expanded = true,
  });

  final String label;
  final VoidCallback? onPressed;
  final AppButtonVariant variant;
  final AppButtonSize size;
  final IconData? icon;
  final bool isLoading;
  final bool expanded;

  double get _height => switch (size) { AppButtonSize.small => 40, AppButtonSize.medium => 52, AppButtonSize.large => 58 };

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final disabled = isLoading || onPressed == null;

    final spinnerColor = switch (variant) {
      AppButtonVariant.outline || AppButtonVariant.text => AppColors.secondary,
      AppButtonVariant.secondary => Colors.black,
      AppButtonVariant.primary || AppButtonVariant.danger => Colors.white,
    };

    Widget child = isLoading
        ? SizedBox(
            width: 20,
            height: 20,
            child: CircularProgressIndicator(strokeWidth: 2.4, color: spinnerColor),
          )
        : Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (icon != null) ...[Icon(icon, size: 18), const SizedBox(width: AppSpacing.sm)],
              Text(label),
            ],
          );

    // The shared theme's ElevatedButtonThemeData hardcodes a 52px minimum
    // height. Small/large AppButtons wrap it in a shorter/taller SizedBox,
    // which — without this override — creates an unsatisfiable constraint
    // (child demands >=52, parent forces exactly 40) that throws during
    // layout and can take the whole frame's render tree down with it.
    final sizeOverride = ButtonStyle(minimumSize: WidgetStatePropertyAll(Size(0, _height)));

    final button = switch (variant) {
      AppButtonVariant.primary => ElevatedButton(style: sizeOverride, onPressed: disabled ? null : onPressed, child: child),
      AppButtonVariant.secondary => ElevatedButton(
          style: ElevatedButton.styleFrom(
            backgroundColor: theme.colorScheme.secondary,
            foregroundColor: theme.colorScheme.onSecondary,
            minimumSize: Size(0, _height),
          ),
          onPressed: disabled ? null : onPressed,
          child: child,
        ),
      AppButtonVariant.outline => OutlinedButton(style: sizeOverride, onPressed: disabled ? null : onPressed, child: child),
      AppButtonVariant.text => TextButton(style: sizeOverride, onPressed: disabled ? null : onPressed, child: child),
      AppButtonVariant.danger => ElevatedButton(
          style: ElevatedButton.styleFrom(
            backgroundColor: theme.colorScheme.error,
            foregroundColor: theme.colorScheme.onError,
            minimumSize: Size(0, _height),
          ),
          onPressed: disabled ? null : onPressed,
          child: child,
        ),
    };

    return SizedBox(
      height: _height,
      width: expanded ? double.infinity : null,
      child: button,
    );
  }
}
