import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:printing/printing.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/transaction_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/utils/statement_pdf_generator.dart';
import '../../core/widgets/premium_card.dart';
import '../../models/transaction.dart';
import '../home/widgets/transaction_tile.dart';

/// Menu → Transaction History: the full payment/disbursal ledger across
/// all loans (`GET /auth/transactions`), grouped by month, with a type
/// filter — not just the 3-item preview shown on the Profile screen.
class TransactionHistoryScreen extends StatefulWidget {
  const TransactionHistoryScreen({super.key});

  @override
  State<TransactionHistoryScreen> createState() => _TransactionHistoryScreenState();
}

class _TransactionHistoryScreenState extends State<TransactionHistoryScreen> {
  TransactionType? _filter;
  late Future<List<AppTransaction>> _future;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  Future<List<AppTransaction>> _load() async {
    final token = AuthProvider.instance.token;
    if (token == null) return const [];
    return TransactionApiService.mine(token);
  }

  Future<void> _refresh() async {
    final next = _load();
    setState(() => _future = next);
    try {
      await next;
    } catch (_) {
      // Already reflected in _future's error state.
    }
  }

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

    return Scaffold(
      appBar: AppBar(
        title: const Text('Transaction History'),
        actions: [
          FutureBuilder<List<AppTransaction>>(
            future: _future,
            builder: (context, snapshot) {
              final all = snapshot.data ?? const [];
              if (all.isEmpty) return const SizedBox.shrink();
              return IconButton(
                icon: const Icon(Icons.picture_as_pdf_outlined),
                tooltip: 'Download Statement',
                onPressed: () => _shareStatement(_filter == null ? all : all.where((t) => t.type == _filter).toList()),
              );
            },
          ),
        ],
      ),
      body: SafeArea(
        child: FutureBuilder<List<AppTransaction>>(
          future: _future,
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError) {
              final message = snapshot.error is ApiException ? (snapshot.error as ApiException).message : 'Could not load your transactions.';
              return Center(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Icon(Icons.error_outline_rounded, size: 48, color: AppColors.error),
                      const SizedBox(height: 12),
                      Text(message, textAlign: TextAlign.center, style: theme.textTheme.bodyMedium),
                      const SizedBox(height: 16),
                      OutlinedButton(onPressed: _refresh, child: const Text('Try Again')),
                    ],
                  ),
                ),
              );
            }

            final all = snapshot.data ?? const [];
            final filtered = _filter == null ? all : all.where((t) => t.type == _filter).toList();

            final grouped = <String, List<AppTransaction>>{};
            for (final t in filtered) {
              final key = DateFormat('MMMM yyyy').format(t.date);
              grouped.putIfAbsent(key, () => []).add(t);
            }

            return RefreshIndicator(
              onRefresh: _refresh,
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
                        ? ListView(
                            physics: const AlwaysScrollableScrollPhysics(),
                            children: [
                              SizedBox(
                                height: 320,
                                child: Center(
                                  child: Text('No transactions in this category.', style: theme.textTheme.bodyMedium),
                                ),
                              ),
                            ],
                          )
                        : ListView(
                            physics: const AlwaysScrollableScrollPhysics(),
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
            );
          },
        ),
      ),
    );
  }
}
