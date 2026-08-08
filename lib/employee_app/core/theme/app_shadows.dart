import 'package:flutter/material.dart';
import '../constants/app_colors.dart';

class AppShadows {
  AppShadows._();

  static List<BoxShadow> card = [
    BoxShadow(color: AppColors.shadow, blurRadius: 24, offset: const Offset(0, 8)),
  ];

  static List<BoxShadow> soft = [
    BoxShadow(color: AppColors.shadow, blurRadius: 12, offset: const Offset(0, 4)),
  ];
}
