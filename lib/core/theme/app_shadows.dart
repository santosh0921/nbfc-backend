import 'package:flutter/material.dart';
import 'app_colors.dart';

/// Soft, low-opacity elevation shadows used across the design system.
/// Premium/enterprise feel: diffuse, low-contrast, no harsh drop shadows.
class AppShadows {
  AppShadows._();

  static const List<BoxShadow> subtle = [
    BoxShadow(color: Color(0x0F10131A), blurRadius: 6, offset: Offset(0, 2)), // textPrimaryLight @ 6%
  ];

  static const List<BoxShadow> soft = [
    BoxShadow(color: Color(0x0D10131A), blurRadius: 20, offset: Offset(0, 8)), // textPrimaryLight @ 5%
  ];

  static const List<BoxShadow> elevated = [
    BoxShadow(color: Color(0x1F10131A), blurRadius: 32, offset: Offset(0, 16)), // textPrimaryLight @ 12%
  ];

  /// Standard card elevation, dark-mode aware (dark surfaces need a
  /// stronger shadow to read as elevated against a dark background).
  static List<BoxShadow> card({required bool isDark}) => [
        BoxShadow(
          color: Colors.black.withValues(alpha: isDark ? 0.24 : 0.05),
          blurRadius: 20,
          offset: const Offset(0, 8),
        ),
      ];

  /// Tinted focus/press glow — derive from [AppColors.primary] at call site
  /// (e.g. `AppColors.primary.withValues(alpha: 0.18)`) rather than adding
  /// new fixed presets here, since the blur/offset stay constant but the
  /// tint varies by context.
  static List<BoxShadow> primaryGlow({double blurRadius = 14, Offset offset = const Offset(0, 6)}) => [
        BoxShadow(color: AppColors.primary.withValues(alpha: 0.18), blurRadius: blurRadius, offset: offset),
      ];
}
