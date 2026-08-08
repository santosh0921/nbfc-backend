import 'package:flutter/material.dart';

/// Closed brand palette for ONEFIN Employee App — yellow / white / black,
/// in the L&T Finance style. Do not use ad-hoc hex values anywhere else in
/// the codebase — extend this file instead.
class AppColors {
  AppColors._();

  // Brand — bright yellow/gold as the single accent, black for CTAs/text
  static const Color primary = Color(0xFFFFC629); // ONEFIN yellow
  static const Color primaryDark = Color(0xFFE0A800);
  static const Color primaryLight = Color(0xFFFFDA70);

  static const Color gold = Color(0xFFFFC629);
  static const Color goldDark = Color(0xFFE0A800);
  static const Color goldLight = Color(0xFFFFDA70);

  static const Color secondary = Color(0xFF14151A); // near-black — CTAs, headings
  static const Color accent = Color(0xFF1E3A8A); // reserved, used sparingly (links)

  // Surfaces
  static const Color background = Color(0xFFFFFFFF);
  static const Color surface = Color(0xFFFFFFFF);
  static const Color surfaceMuted = Color(0xFFF7F7F9);
  static const Color darkBackground = Color(0xFF0E0E11);
  static const Color darkSurfaceBase = Color(0xFF1B1C21);

  // Status
  static const Color success = Color(0xFF16A34A);
  static const Color warning = Color(0xFFF59E0B);
  static const Color danger = Color(0xFFDC2626);
  static const Color info = Color(0xFF2563EB);

  // Text
  static const Color textPrimary = Color(0xFF14151A);
  static const Color textSecondary = Color(0xFF6B7280);
  static const Color textOnPrimary = Color(0xFF14151A); // yellow buttons use dark text
  static const Color darkTextPrimary = Color(0xFFF4F4F5);
  static const Color darkTextSecondary = Color(0xFFA1A1AA);

  // Derived / state
  static Color border = const Color(0xFF14151A).withValues(alpha: 0.10);
  static Color divider = const Color(0xFF14151A).withValues(alpha: 0.07);
  static Color disabled = const Color(0xFF14151A).withValues(alpha: 0.35);
  static Color scrim = const Color(0xFF000000).withValues(alpha: 0.45);
  static Color shadow = const Color(0xFF14151A).withValues(alpha: 0.08);

  static Color glassFill = const Color(0xFFFFFFFF).withValues(alpha: 0.55);
  static Color glassBorder = const Color(0xFF14151A).withValues(alpha: 0.08);
  static Color glassHighlight = const Color(0xFFFFFFFF).withValues(alpha: 0.7);

  static const List<Color> heroGradient = [primaryLight, primary];
  static const List<Color> goldGradient = [primary, primaryDark];
}
