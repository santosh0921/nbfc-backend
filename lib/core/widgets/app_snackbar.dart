import 'package:flutter/material.dart';
import '../theme/app_colors.dart';

/// App-wide success/error snackbar — green on success (matching
/// [AppColors.success], the same tone used for "Active"/"Verified"
/// pills elsewhere in the app) so a completed action reads as
/// unambiguously successful rather than a plain grey toast.
class AppSnackBar {
  AppSnackBar._();

  static void showSuccess(BuildContext context, String message) {
    _show(context, message: message, color: AppColors.success, icon: Icons.check_circle_rounded);
  }

  static void showError(BuildContext context, String message) {
    _show(context, message: message, color: AppColors.error, icon: Icons.error_rounded);
  }

  static void _show(
    BuildContext context, {
    required String message,
    required Color color,
    required IconData icon,
  }) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(
        SnackBar(
          behavior: SnackBarBehavior.floating,
          backgroundColor: color,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          margin: const EdgeInsets.all(16),
          content: Row(
            children: [
              Icon(icon, color: Colors.white, size: 20),
              const SizedBox(width: 10),
              Expanded(
                child: Text(message, style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w600)),
              ),
            ],
          ),
        ),
      );
  }
}
