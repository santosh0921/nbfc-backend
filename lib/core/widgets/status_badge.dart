import 'package:flutter/material.dart';
import '../theme/app_colors.dart';
import '../theme/app_radius.dart';

enum StatusTone { success, warning, error, info, neutral }

/// Small pill label for a status/priority word (e.g. "Approved", "Pending",
/// "Overdue") — the one place in the design system where a pill shape is
/// correct. Uses restrained tinted backgrounds, not solid fills, to stay
/// premium rather than loud.
class StatusBadge extends StatelessWidget {
  const StatusBadge({super.key, required this.label, this.tone = StatusTone.neutral});

  final String label;
  final StatusTone tone;

  @override
  Widget build(BuildContext context) {
    final Color color;
    switch (tone) {
      case StatusTone.success:
        color = AppColors.success;
        break;
      case StatusTone.warning:
        color = AppColors.warning;
        break;
      case StatusTone.error:
        color = AppColors.error;
        break;
      case StatusTone.info:
        color = AppColors.secondary;
        break;
      case StatusTone.neutral:
        color = AppColors.textSecondaryOf(context);
        break;
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(AppRadius.pill),
      ),
      child: Text(
        label,
        style: TextStyle(color: color, fontWeight: FontWeight.w600, fontSize: 12),
      ),
    );
  }
}
