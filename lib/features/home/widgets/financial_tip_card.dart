import 'package:flutter/material.dart';
import '../../../core/theme/app_radius.dart';
import '../../../models/offer_banner.dart';

class FinancialTipCard extends StatelessWidget {
  final FinancialTip tip;

  const FinancialTipCard({super.key, required this.tip});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      width: 220,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppRadius.lg),
        gradient: LinearGradient(colors: [tip.gradient.first.withValues(alpha: 0.16), tip.gradient.last.withValues(alpha: 0.06)]),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.lightbulb_rounded, color: tip.gradient.last),
          const SizedBox(height: 12),
          Text(tip.title, style: theme.textTheme.titleSmall, maxLines: 2),
          const SizedBox(height: 6),
          Text(tip.summary, style: theme.textTheme.bodySmall, maxLines: 2),
          const Spacer(),
          Text(tip.readTime, style: theme.textTheme.labelSmall),
        ],
      ),
    );
  }
}
