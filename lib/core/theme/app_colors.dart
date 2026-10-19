import 'package:flutter/material.dart';

/// Brand palette for OneFinance. Single source of truth for color tokens
/// used across light and dark themes.
///
/// Brand identity: warm gold/champagne is the primary signal, carried on a
/// white/cream canvas — gold is an accent and trust cue, not a wash across
/// every surface. `primary` drives buttons/focus states/selected nav via
/// [AppTheme], so retinting it here is what actually re-skins the app;
/// avoid reading the old blue-forward `secondary`/`gradientBluePrimary`
/// tokens for anything meant to read as "brand" going forward.
class AppColors {
  AppColors._();

  static const Color primary = Color(0xFFF5B800); // Premium warm gold
  static const Color primaryDark = Color(0xFFD99A00); // Deep gold
  static const Color primaryLight = Color(0xFFFFF4CC); // Soft champagne

  // Kept for screens/flows not yet migrated off the legacy blue accent —
  // new UI should reach for `primary` (gold) instead.
  static const Color secondary = Color(0xFF0057FF);
  static const Color secondaryDark = Color(0xFF0041C4);

  static const Color success = Color(0xFF2F9E5B); // restrained premium green
  static const Color warning = Color(0xFFC77E11); // restrained amber
  static const Color error = Color(0xFFC1443A); // restrained red
  static const Color info = Color(0xFF0057FF);

  static const Color backgroundLight = Color(0xFFFAFAF7);
  static const Color surfaceLight = Color(0xFFFFFFFF);
  static const Color backgroundDark = Color(0xFF14120C);
  static const Color surfaceDark = Color(0xFF1E1B12);

  static const Color textPrimaryLight = Color(0xFF171717);
  static const Color textSecondaryLight = Color(0xFF6B6B6B);
  static const Color textPrimaryDark = Color(0xFFF3F1E9);
  static const Color textSecondaryDark = Color(0xFFB9B2A0);

  static const Color borderLight = Color(0xFFEAE7DE);
  static const Color borderDark = Color(0xFF3A3527);

  // Text/icon color to place ON TOP of a solid `primary` (gold) surface —
  // gold is light, so it needs the dark text color, not white.
  static const Color onPrimary = Color(0xFF171717);

  // Gradient presets used across hero banners and cards.
  static const List<Color> gradientGold = [
    Color(0xFFF5B800),
    Color(0xFFD99A00),
  ];

  static const List<Color> gradientChampagne = [
    Color(0xFFFFF4CC),
    Color(0xFFF5B800),
  ];

  static const List<Color> gradientBluePrimary = [
    Color(0xFF0057FF),
    Color(0xFF3D7BFF),
  ];

  static const List<Color> gradientNavy = [
    Color(0xFF0B1B4D),
    Color(0xFF0057FF),
  ];

  static const List<Color> gradientEmerald = [
    Color(0xFF0FBE7A),
    Color(0xFF17C964),
  ];

  static const List<Color> gradientPlum = [
    Color(0xFF6C2BD9),
    Color(0xFF9C4DFF),
  ];

  static const List<Color> gradientSunset = [
    Color(0xFFFF7A59),
    Color(0xFFFFB020),
  ];

  // Context-aware accessors — for the common case of "this color should
  // just follow the current theme". Screens that used the raw `*Light`
  // constants directly (with no dark-mode branch at all) is what made
  // large parts of the app unreadable in dark mode: light-cream surfaces
  // and near-black text stayed fixed regardless of theme, so dark mode
  // showed glowing white/cream boxes with invisible-on-dark text. Prefer
  // these over the raw constants in new code; the raw constants stay for
  // call sites (AppTheme, PremiumCard, ...) that already branch on
  // `isDark` explicitly.
  static bool _isDark(BuildContext context) => Theme.of(context).brightness == Brightness.dark;
  static Color textPrimaryOf(BuildContext context) => _isDark(context) ? textPrimaryDark : textPrimaryLight;
  static Color textSecondaryOf(BuildContext context) => _isDark(context) ? textSecondaryDark : textSecondaryLight;
  static Color backgroundOf(BuildContext context) => _isDark(context) ? backgroundDark : backgroundLight;
  static Color surfaceOf(BuildContext context) => _isDark(context) ? surfaceDark : surfaceLight;
  static Color borderOf(BuildContext context) => _isDark(context) ? borderDark : borderLight;
}
