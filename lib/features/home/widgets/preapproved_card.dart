import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/widgets/gradient_container.dart';

class PreApprovedCard extends StatelessWidget {
  final double amount;
  final VoidCallback onApply;

  const PreApprovedCard({super.key, required this.amount, required this.onApply});

  @override
  Widget build(BuildContext context) {
    final formatted = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0).format(amount);
    return GradientContainer(
      colors: AppColors.gradientGold,
      radius: 24,
      padding: const EdgeInsets.all(22),
      shine: true,
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    const Icon(Icons.workspace_premium_rounded, color: AppColors.onPrimary, size: 18),
                    const SizedBox(width: 6),
                    Text(
                      'Congratulations!',
                      style: Theme.of(context).textTheme.labelLarge?.copyWith(color: AppColors.onPrimary),
                    ),
                  ],
                ),
                const SizedBox(height: 10),
                Text(
                  'You\'re pre-approved for',
                  style: Theme.of(context)
                      .textTheme
                      .bodyMedium
                      ?.copyWith(color: AppColors.onPrimary.withValues(alpha: 0.75)),
                ),
                Text(
                  formatted,
                  style: Theme.of(context)
                      .textTheme
                      .displaySmall
                      ?.copyWith(color: AppColors.onPrimary, fontWeight: FontWeight.w800),
                ),
                const SizedBox(height: 14),
                ElevatedButton(
                  onPressed: onApply,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: Colors.white,
                    foregroundColor: AppColors.primaryDark,
                    padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
                  ),
                  child: const Text('Apply Now'),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
