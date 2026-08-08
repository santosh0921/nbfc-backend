import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/widgets/premium_card.dart';
import '../../../models/emi_schedule.dart';

/// Home dashboard "Next EMI Due" card, backed by the real
/// `GET /auth/dashboard/emi-summary` aggregate across all the customer's
/// disbursed loans. Only rendered by the caller when
/// [summary.hasUpcomingEmi] is true — no jarring empty state shown when
/// the customer has no active EMI schedule yet.
class EmiDashboardSummaryCard extends StatelessWidget {
  const EmiDashboardSummaryCard({super.key, required this.summary, required this.onTap});

  final EmiDashboardSummary summary;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final dueDate = summary.nextDueDate;
    final daysLeft = dueDate?.difference(DateTime.now()).inDays;
    final dueLabel = daysLeft == null
        ? ''
        : (daysLeft <= 0 ? 'Due Today' : (daysLeft == 1 ? 'Due Tomorrow' : 'Due in $daysLeft days'));
    final amount = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0).format(summary.nextDueAmount);

    return PremiumCard(
      onTap: onTap,
      child: Row(
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: AppColors.warning.withValues(alpha: 0.14),
              borderRadius: BorderRadius.circular(AppRadius.md),
            ),
            child: const Icon(Icons.event_available_rounded, color: AppColors.warning),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text('Next EMI Due', style: theme.textTheme.titleSmall),
                    if (dueLabel.isNotEmpty) ...[
                      const SizedBox(width: 8),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                        decoration: BoxDecoration(
                          color: AppColors.warning.withValues(alpha: 0.14),
                          borderRadius: BorderRadius.circular(AppRadius.pill),
                        ),
                        child: Text(
                          dueLabel,
                          style: theme.textTheme.labelSmall?.copyWith(color: AppColors.warning, fontWeight: FontWeight.w700),
                        ),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 4),
                Text(amount, style: theme.textTheme.headlineSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                if (summary.totalActiveLoans > 1)
                  Text('Across ${summary.totalActiveLoans} active loans', style: theme.textTheme.bodySmall),
              ],
            ),
          ),
          const Icon(Icons.chevron_right_rounded, color: AppColors.textSecondaryLight),
        ],
      ),
    );
  }
}
