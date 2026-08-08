import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';
import 'app_colors.dart';

/// Typography scale built on Inter (body/UI) + Manrope (display/headings)
/// for a premium, editorial fintech feel.
class AppTextStyles {
  AppTextStyles._();

  static TextTheme textTheme(Brightness brightness) {
    final Color primaryColor =
        brightness == Brightness.dark ? AppColors.textPrimaryDark : AppColors.textPrimaryLight;
    final Color secondaryColor =
        brightness == Brightness.dark ? AppColors.textSecondaryDark : AppColors.textSecondaryLight;

    TextStyle display(double size, FontWeight weight, {double? height, double? letterSpacing}) =>
        GoogleFonts.manrope(
          fontSize: size,
          fontWeight: weight,
          color: primaryColor,
          height: height,
          letterSpacing: letterSpacing,
        );

    TextStyle body(double size, FontWeight weight, {Color? color, double? height}) => GoogleFonts.inter(
          fontSize: size,
          fontWeight: weight,
          color: color ?? primaryColor,
          height: height,
        );

    return TextTheme(
      displayLarge: display(40, FontWeight.w800, height: 1.1, letterSpacing: -0.5),
      displayMedium: display(32, FontWeight.w800, height: 1.15, letterSpacing: -0.4),
      displaySmall: display(28, FontWeight.w700, height: 1.2),
      headlineLarge: display(26, FontWeight.w700, height: 1.2),
      headlineMedium: display(22, FontWeight.w700, height: 1.25),
      headlineSmall: display(19, FontWeight.w700, height: 1.3),
      titleLarge: display(18, FontWeight.w700),
      titleMedium: body(16, FontWeight.w600),
      titleSmall: body(14, FontWeight.w600),
      bodyLarge: body(16, FontWeight.w400, height: 1.5),
      bodyMedium: body(14, FontWeight.w400, height: 1.45, color: secondaryColor),
      bodySmall: body(12, FontWeight.w400, height: 1.4, color: secondaryColor),
      labelLarge: body(14, FontWeight.w600),
      labelMedium: body(12, FontWeight.w600, color: secondaryColor),
      labelSmall: body(11, FontWeight.w600, color: secondaryColor),
    );
  }
}
