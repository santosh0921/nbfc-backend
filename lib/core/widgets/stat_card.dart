import 'package:flutter/material.dart';
import '../theme/app_colors.dart';
import 'glass_card.dart';
import 'premium_card.dart';

/// A single big-number metric tile — "24 Pending", "₹12.4L Outstanding" —
/// used in dashboard overview rows. Set [highlight] for the one stat per
/// row that should draw the eye (renders as a [GlassCard] instead of a
/// plain [PremiumCard]).
class StatCard extends StatelessWidget {
  const StatCard({
    super.key,
    required this.value,
    required this.label,
    this.icon,
    this.highlight = false,
  });

  final String value;
  final String label;
  final IconData? icon;
  final bool highlight;

  @override
  Widget build(BuildContext context) {
    final content = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (icon != null) ...[
          Icon(icon, size: 20, color: AppColors.primaryDark),
          const SizedBox(height: 10),
        ],
        Text(
          value,
          style: const TextStyle(fontSize: 26, fontWeight: FontWeight.w800, height: 1.1),
        ),
        const SizedBox(height: 4),
        Text(label, style: const TextStyle(fontSize: 13, color: AppColors.textSecondaryLight)),
      ],
    );

    return highlight
        ? GlassCard(padding: const EdgeInsets.all(18), child: content)
        : PremiumCard(padding: const EdgeInsets.all(18), child: content);
  }
}
