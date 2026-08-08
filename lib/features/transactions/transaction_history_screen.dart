import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:printing/printing.dart';
import '../../core/theme/app_colors.dart';
import '../../core/utils/statement_pdf_generator.dart';
import '../../core/widgets/premium_card.dart';
import '../../mock/mock_data.dart';
import '../../models/transaction.dart';
import '../home/widgets/transaction_tile.dart';

/// Menu → Transaction History: the full payment/disbursal/fee ledger
/// across all loans, grouped by month, with a type filter — not just the
/// 3-item preview shown on the Profile screen.
class TransactionHistoryScreen extends StatefulWidget {
  const TransactionHistoryScreen({super.key});

  @override
  State<TransactionHistoryScreen> createState() => _TransactionHistoryScreenState();
}

class _TransactionHistoryScreenState extends State<TransactionHistoryScreen> {
  TransactionType? _filter;

  Future<void> _shareStatement(List<AppTransaction> filtered) async {
    final bytes = await StatementPdfGenerator.buildTransactionsStatement(filtered);
    await Printing.sharePdf(bytes: bytes, filename: 'transaction_history.pdf');
  }

  static const _filters = [
    (null, 'All'),
    (TransactionType.emiPayment, 'EMI'),
    (TransactionType.partPayment, 'Part Payment'),
    (TransactionType.disbursal, 'Disbursal'),
    (TransactionType.fee, 'Fees'),
  ];

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final all = MockData.recentTransactions;
    final filtered = _filter == null ? all : all.where((t) => t.type == _filter).toList();

    final grouped = <String, List<AppTransaction>>{};
    for (final t in filtered) {
      final key = DateFormat('MMMM yyyy').format(t.date);
      grouped.putIfAbsent(key, () => []).add(t);
    }

    return Scaffold(
      appBar: AppBar(
        title: const Text('Transaction History'),
        actions: [
          if (filtered.isNotEmpty)
            IconButton(
              icon: const Icon(Icons.picture_as_pdf_outlined),
              tooltip: 'Download Statement',
              onPressed: () => _shareStatement(filtered),
            ),
        ],
      ),
      body: SafeArea(
        child: Column(
          children: [
            SizedBox(
              height: 44,
              child: ListView.separated(
                padding: const EdgeInsets.fromLTRB(20, 8, 20, 4),
                scrollDirection: Axis.horizontal,
                itemCount: _filters.length,
                separatorBuilder: (_, __) => const SizedBox(width: 8),
                itemBuilder: (context, i) {
                  final (type, label) = _filters[i];
                  final selected = _filter == type;
                  return ChoiceChip(
                    label: Text(label),
                    selected: selected,
                    onSelected: (_) => setState(() => _filter = type),
                    selectedColor: AppColors.primary.withValues(alpha: 0.16),
                    labelStyle: TextStyle(color: selected ? AppColors.primary : null, fontWeight: selected ? FontWeight.w700 : FontWeight.w500),
                    side: BorderSide(color: selected ? AppColors.primary : AppColors.borderLight),
                  );
                },
              ),
            ),
            Expanded(
              child: filtered.isEmpty
                  ? Center(
                      child: Text('No transactions in this category.', style: theme.textTheme.bodyMedium),
                    )
                  : ListView(
                      padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
                      children: [
                        for (final entry in grouped.entries) ...[
                          Padding(
                            padding: const EdgeInsets.only(bottom: 8, top: 12),
                            child: Text(entry.key, style: theme.textTheme.titleSmall?.copyWith(color: AppColors.textSecondaryLight)),
                          ),
                          PremiumCard(
                            padding: const EdgeInsets.symmetric(horizontal: 16),
                            child: Column(
                              children: [
                                for (int i = 0; i < entry.value.length; i++) ...[
                                  TransactionTile(transaction: entry.value[i]),
                                  if (i != entry.value.length - 1) const Divider(height: 1),
                                ],
                              ],
                            ),
                          ),
                        ],
                      ],
                    ),
            ),
          ],
        ),
      ),
    );
  }
}
