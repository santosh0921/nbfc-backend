import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../models/transaction.dart';

class TransactionTile extends StatelessWidget {
  final AppTransaction transaction;

  const TransactionTile({super.key, required this.transaction});

  IconData get _icon {
    switch (transaction.type) {
      case TransactionType.emiPayment:
        return Icons.receipt_long_rounded;
      case TransactionType.disbursal:
        return Icons.account_balance_wallet_rounded;
      case TransactionType.fee:
        return Icons.request_quote_rounded;
      case TransactionType.refund:
        return Icons.undo_rounded;
      case TransactionType.partPayment:
        return Icons.savings_rounded;
    }
  }

  bool get _isCredit => transaction.type == TransactionType.disbursal || transaction.type == TransactionType.refund;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final amount = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0).format(transaction.amount);
    final date = DateFormat('d MMM, h:mm a').format(transaction.date);

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Row(
        children: [
          Container(
            width: 42,
            height: 42,
            decoration: BoxDecoration(
              color: isDark ? AppColors.surfaceDark : const Color(0xFFEFF3FA),
              borderRadius: BorderRadius.circular(AppRadius.sm),
            ),
            child: Icon(_icon, color: AppColors.primary, size: 20),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(transaction.description, style: theme.textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                Text(date, style: theme.textTheme.bodySmall),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Text(
                '${_isCredit ? '+' : '-'} $amount',
                style: theme.textTheme.titleSmall?.copyWith(color: _isCredit ? AppColors.success : AppColors.error),
              ),
              if (transaction.status != TransactionStatus.success) ...[
                const SizedBox(height: 2),
                Text(
                  transaction.status == TransactionStatus.pending ? 'Pending' : 'Failed',
                  style: theme.textTheme.labelSmall?.copyWith(
                    color: transaction.status == TransactionStatus.pending ? AppColors.warning : AppColors.error,
                  ),
                ),
              ],
            ],
          ),
        ],
      ),
    );
  }
}
