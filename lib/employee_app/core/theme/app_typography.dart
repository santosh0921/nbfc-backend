import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';
import '../constants/app_colors.dart';

class AppTypography {
  AppTypography._();

  static TextTheme build(Color base) {
    return GoogleFonts.interTextTheme(
      TextTheme(
        displayLarge: TextStyle(fontSize: 40, fontWeight: FontWeight.w700, letterSpacing: -0.5, color: base),
        displayMedium: TextStyle(fontSize: 34, fontWeight: FontWeight.w700, letterSpacing: -0.4, color: base),
        displaySmall: TextStyle(fontSize: 28, fontWeight: FontWeight.w700, letterSpacing: -0.3, color: base),
        headlineLarge: TextStyle(fontSize: 26, fontWeight: FontWeight.w700, letterSpacing: -0.2, color: base),
        headlineMedium: TextStyle(fontSize: 22, fontWeight: FontWeight.w600, letterSpacing: -0.2, color: base),
        headlineSmall: TextStyle(fontSize: 20, fontWeight: FontWeight.w600, color: base),
        titleLarge: TextStyle(fontSize: 18, fontWeight: FontWeight.w600, color: base),
        titleMedium: TextStyle(fontSize: 16, fontWeight: FontWeight.w600, color: base),
        titleSmall: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: base),
        bodyLarge: TextStyle(fontSize: 16, fontWeight: FontWeight.w400, color: base),
        bodyMedium: TextStyle(fontSize: 14, fontWeight: FontWeight.w400, color: base),
        bodySmall: TextStyle(fontSize: 12, fontWeight: FontWeight.w400, color: base),
        labelLarge: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, letterSpacing: 0.1, color: base),
        labelMedium: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, letterSpacing: 0.1, color: base),
        labelSmall: TextStyle(fontSize: 11, fontWeight: FontWeight.w600, letterSpacing: 0.2, color: base),
      ),
    );
  }

  static TextTheme get light => build(AppColors.textPrimary);
  static TextTheme get dark => build(AppColors.darkTextPrimary);
}
