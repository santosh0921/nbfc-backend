import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/emi_api_service.dart';
import '../../core/network/loan_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../../models/emi_schedule.dart';
import '../../models/loan_application.dart';

/// The customer-app bottom-nav tab that replaced Financial Marketplace —
/// a cross-loan EMI overview (aggregate summary via the existing
/// GET /auth/dashboard/emi-summary + a tap-through list of every loan,
/// each opening its own full amortization schedule). Marketplace was a
/// browsing feature with no real backend behind it; this is the
/// customer's actual loan/repayment information, which is what a lending
/// app's primary nav should surface.
class EmiPaymentsScreen extends StatefulWidget {
  const EmiPaymentsScreen({super.key});

  @override
  State<EmiPaymentsScreen> createState() => _EmiPaymentsScreenState();
}

class _EmiPaymentsScreenState extends State<EmiPaymentsScreen> {
  late Future<(EmiDashboardSummary, List<LoanApplication>)> _future;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  Future<(EmiDashboardSummary, List<LoanApplication>)> _load() async {
    final token = AuthProvider.instance.token;
    if (token == null) {
      return (const EmiDashboardSummary(hasUpcomingEmi: false, nextDueAmount: 0, totalPaidAcrossLoans: 0, totalRemainingAcrossLoans: 0, totalActiveLoans: 0), <LoanApplication>[]);
    }
    final results = await Future.wait([
      EmiApiService.dashboardSummary(token),
      LoanApiService.mine(token),
    ]);
    return (results[0] as EmiDashboardSummary, results[1] as List<LoanApplication>);
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

  String _compact(double amount) {
    if (amount >= 10000000) return '₹${(amount / 10000000).toStringAsFixed(amount % 10000000 == 0 ? 0 : 1)}Cr';
    if (amount >= 100000) return '₹${(amount / 100000).toStringAsFixed(amount % 100000 == 0 ? 0 : 1)}L';
    if (amount >= 1000) return '₹${(amount / 1000).toStringAsFixed(amount % 1000 == 0 ? 0 : 1)}K';
    return '₹${amount.toStringAsFixed(0)}';
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('EMI Payments')),
      body: SafeArea(
        child: FutureBuilder<(EmiDashboardSummary, List<LoanApplication>)>(
          future: _future,
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError) {
              final message = snapshot.error is ApiException ? (snapshot.error as ApiException).message : 'Could not load your EMI details.';
              return Center(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(message, textAlign: TextAlign.center),
                      const SizedBox(height: 12),
                      OutlinedButton(onPressed: _refresh, child: const Text('Retry')),
                    ],
                  ),
                ),
              );
            }

            final (summary, loans) = snapshot.data!;
            final activeLoans = loans.where((l) => l.status == 'disbursed').toList();

            return RefreshIndicator(
              onRefresh: _refresh,
              child: ListView(
                padding: const EdgeInsets.all(20),
                children: [
                  if (summary.hasUpcomingEmi) ...[
                    PremiumCard(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              Container(
                                width: 44,
                                height: 44,
                                decoration: BoxDecoration(
                                  color: AppColors.primary.withValues(alpha: 0.1),
                                  borderRadius: BorderRadius.circular(AppRadius.sm),
                                ),
                                child: const Icon(Icons.event_available_rounded, color: AppColors.primary),
                              ),
                              const SizedBox(width: 12),
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text('Next EMI Due', style: theme.textTheme.bodySmall),
                                    Text(
                                      '₹${summary.nextDueAmount.toStringAsFixed(0)}',
                                      style: theme.textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w700),
                                    ),
                                  ],
                                ),
                              ),
                              if (summary.nextDueDate != null)
                                Text(
                                  DateFormat('d MMM').format(summary.nextDueDate!),
                                  style: theme.textTheme.titleSmall,
                                ),
                            ],
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 16),
                  ],
                  Row(
                    children: [
                      Expanded(
                        child: _StatTile(label: 'Total Paid', value: _compact(summary.totalPaidAcrossLoans), color: AppColors.success),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: _StatTile(label: 'Remaining', value: _compact(summary.totalRemainingAcrossLoans), color: AppColors.warning),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: _StatTile(label: 'Active Loans', value: '${summary.totalActiveLoans}', color: AppColors.primary),
                      ),
                    ],
                  ),
                  const SizedBox(height: 24),
                  Text('Your Loans', style: theme.textTheme.titleMedium),
                  const SizedBox(height: 12),
                  if (loans.isEmpty)
                    Padding(
                      padding: const EdgeInsets.symmetric(vertical: 40),
                      child: Center(
                        child: Column(
                          children: [
                            const Icon(Icons.receipt_long_outlined, size: 48, color: AppColors.textSecondaryLight),
                            const SizedBox(height: 12),
                            Text('No loan applications yet.', style: theme.textTheme.titleMedium),
                          ],
                        ),
                      ),
                    )
                  else
                    for (final loan in loans) _LoanEmiTile(loan: loan, isActive: activeLoans.contains(loan)),
                ],
              ),
            );
          },
        ),
      ),
    );
  }
}

class _StatTile extends StatelessWidget {
  const _StatTile({required this.label, required this.value, required this.color});

  final String label;
  final String value;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 14, horizontal: 10),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppRadius.md),
        border: Border.all(color: color.withValues(alpha: 0.2)),
      ),
      child: Column(
        children: [
          Text(value, style: TextStyle(color: color, fontSize: 16, fontWeight: FontWeight.w800)),
          const SizedBox(height: 4),
          Text(label, style: Theme.of(context).textTheme.bodySmall, textAlign: TextAlign.center),
        ],
      ),
    );
  }
}

class _LoanEmiTile extends StatelessWidget {
  const _LoanEmiTile({required this.loan, required this.isActive});

  final LoanApplication loan;
  final bool isActive;

  @override
  Widget build(BuildContext context) {
    final info = LoanStatusInfo.forStatus(loan.status);
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: PremiumCard(
        onTap: isActive ? () => context.push('/emi-schedule', extra: '${loan.id}') : null,
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              alignment: Alignment.center,
              decoration: BoxDecoration(
                color: AppColors.primary.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppRadius.sm),
              ),
              child: const Icon(Icons.account_balance_rounded, color: AppColors.primary),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(loan.category, style: Theme.of(context).textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                  const SizedBox(height: 2),
                  Text(
                    isActive ? 'NBFC-APP-${loan.id} · Tap for EMI schedule' : 'NBFC-APP-${loan.id} · ${info.label}',
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondaryLight),
                  ),
                ],
              ),
            ),
            if (isActive) const Icon(Icons.chevron_right_rounded, color: AppColors.textSecondaryLight),
          ],
        ),
      ),
    );
  }
}
