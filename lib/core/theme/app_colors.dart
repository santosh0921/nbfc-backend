import 'package:flutter/material.dart';

/// Brand palette for NBFC Premium. Single source of truth for color tokens
/// used across light and dark themes.
class AppColors {
  AppColors._();

  static const Color primary = Color(0xFF0057FF);
  static const Color primaryDark = Color(0xFF0041C4);
  static const Color primaryLight = Color(0xFF5C8CFF);

  static const Color secondary = Color(0xFFF6C445);
  static const Color secondaryDark = Color(0xFFD9A61E);

  static const Color success = Color(0xFF17C964);
  static const Color warning = Color(0xFFFFB020);
  static const Color error = Color(0xFFE53935);
  static const Color info = Color(0xFF0057FF);

  static const Color backgroundLight = Color(0xFFF7F9FC);
  static const Color surfaceLight = Color(0xFFFFFFFF);
  static const Color backgroundDark = Color(0xFF0B0E14);
  static const Color surfaceDark = Color(0xFF161A22);

  static const Color textPrimaryLight = Color(0xFF10131A);
  static const Color textSecondaryLight = Color(0xFF5B6472);
  static const Color textPrimaryDark = Color(0xFFF3F5F8);
  static const Color textSecondaryDark = Color(0xFF9AA4B2);

  static const Color borderLight = Color(0xFFE7EBF1);
  static const Color borderDark = Color(0xFF262C38);

  // Gradient presets used across hero banners and cards.
  static const List<Color> gradientBluePrimary = [
    Color(0xFF0057FF),
    Color(0xFF3D7BFF),
  ];

  static const List<Color> gradientNavy = [
    Color(0xFF0B1B4D),
    Color(0xFF0057FF),
  ];

  static const List<Color> gradientGold = [
    Color(0xFFF6C445),
    Color(0xFFE79A2A),
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
}
