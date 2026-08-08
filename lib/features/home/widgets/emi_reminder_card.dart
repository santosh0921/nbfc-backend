import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/widgets/premium_card.dart';
import '../../../models/active_loan.dart';

class EmiReminderCard extends StatelessWidget {
  final ActiveLoan loan;
  final VoidCallback onPayNow;

  const EmiReminderCard({super.key, required this.loan, required this.onPayNow});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final daysLeft = loan.nextEmiDate.difference(DateTime.now()).inDays;
    final dueLabel = daysLeft <= 0 ? 'Due Today' : (daysLeft == 1 ? 'Due Tomorrow' : 'Due in $daysLeft days');
    final amount = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0).format(loan.nextEmiAmount);

    return PremiumCard(
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
                    Text('Upcoming EMI', style: theme.textTheme.titleSmall),
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
                ),
                const SizedBox(height: 4),
                Text(amount, style: theme.textTheme.headlineSmall),
                Text(loan.productName, style: theme.textTheme.bodySmall),
              ],
            ),
          ),
          ElevatedButton(onPressed: onPayNow, child: const Text('Pay Now')),
        ],
      ),
    );
  }
}
