import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:printing/printing.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/emi_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/utils/statement_pdf_generator.dart';
import '../../core/widgets/premium_card.dart';
import '../../models/emi_schedule.dart';
import 'widgets/emi_calendar_view.dart';

/// Menu → EMI Schedule: the real backend-computed amortization schedule
/// for a disbursed loan (`GET /auth/loans/:id/emi-schedule`), reached
/// from the Menu, or "View EMI Schedule" on a disbursed loan card in
/// My Loans (`my_loans_screen.dart`). `loanId` is the real backend loan
/// ID (an int, passed through the router as a string via `extra`).
class EmiScheduleScreen extends StatefulWidget {
  const EmiScheduleScreen({super.key, this.loanId});

  final String? loanId;

  @override
  State<EmiScheduleScreen> createState() => _EmiScheduleScreenState();
}

class _EmiScheduleScreenState extends State<EmiScheduleScreen> {
  late Future<EmiSchedule> _future;
  bool _paying = false;
  bool _showCalendar = false;

  int? get _loanId => int.tryParse(widget.loanId ?? '');

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  Future<EmiSchedule> _load() async {
    final id = _loanId;
    final token = AuthProvider.instance.token;
    if (id == null || token == null) {
      throw ApiException('This loan could not be found.');
    }
    return EmiApiService.schedule(token, id);
  }

  Future<void> _refresh() async {
    final next = _load();
    setState(() => _future = next);
    // The FutureBuilder below is what actually surfaces a failure (via
    // its error state) — awaiting here is only so callers like
    // RefreshIndicator know when the refresh finished, so a rejection
    // must be swallowed rather than left to propagate as an unhandled
    // exception on every retry tap.
    try {
      await next;
    } catch (_) {
      // Already reflected in _future's error state; nothing more to do.
    }
  }

  Future<void> _payNow(EmiInstallment installment) async {
    final id = _loanId;
    final token = AuthProvider.instance.token;
    if (id == null || token == null || _paying) return;
    setState(() => _paying = true);
    try {
      await EmiApiService.payInstallment(token, id, installment.id);
      if (!mounted) return;
      await _refresh();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Installment #${installment.installmentNumber} paid successfully.')),
      );
    } on ApiException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
    } finally {
      if (mounted) setState(() => _paying = false);
    }
  }

  Future<void> _shareSchedule(EmiSchedule schedule) async {
    final bytes = await StatementPdfGenerator.buildEmiScheduleFromInstallments(
      loanId: schedule.loanId,
      tenureMonths: schedule.tenureMonths,
      interestRatePercent: schedule.interestRatePercent,
      installments: schedule.installments
          .map((i) => (
                i.installmentNumber,
                i.dueDate,
                i.amount,
                i.principalComponent,
                i.interestComponent,
                i.status,
              ))
          .toList(),
    );
    await Printing.sharePdf(bytes: bytes, filename: 'emi_schedule_${schedule.loanId}.pdf');
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('EMI Schedule'),
        actions: [
          IconButton(
            icon: Icon(_showCalendar ? Icons.view_list_rounded : Icons.calendar_month_rounded),
            tooltip: _showCalendar ? 'List View' : 'Calendar View',
            onPressed: () => setState(() => _showCalendar = !_showCalendar),
          ),
          FutureBuilder<EmiSchedule>(
            future: _future,
            builder: (context, snapshot) {
              if (!snapshot.hasData) return const SizedBox.shrink();
              return IconButton(
                icon: const Icon(Icons.picture_as_pdf_outlined),
                tooltip: 'Download Statement',
                onPressed: () => _shareSchedule(snapshot.data!),
              );
            },
          ),
        ],
      ),
      body: SafeArea(
        child: FutureBuilder<EmiSchedule>(
          future: _future,
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError || !snapshot.hasData) {
              final message = snapshot.error is ApiException ? (snapshot.error as ApiException).message : 'Could not load the EMI schedule.';
              return _ErrorState(message: message, onRetry: _refresh);
            }
            final schedule = snapshot.data!;
            if (schedule.installments.isEmpty) {
              return _EmptyState(onRefresh: _refresh);
            }
            final nextPendingId = schedule.installments
                .where((i) => i.status != 'paid')
                .fold<EmiInstallment?>(null, (next, i) => next == null || i.dueDate.isBefore(next.dueDate) ? i : next)
                ?.id;

            return RefreshIndicator(
              onRefresh: _refresh,
              child: ListView(
                padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
                children: [
                  _ScheduleHeader(schedule: schedule),
                  const SizedBox(height: 24),
                  if (_showCalendar) ...[
                    EmiCalendarView(
                      installments: schedule.installments,
                      onSelectInstallment: (installment) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(
                            content: Text(
                              'Installment #${installment.installmentNumber} — ${_currency.format(installment.amount)} '
                              '(${EmiStatusInfo.forStatus(installment.status).label}), due ${_dateFmt.format(installment.dueDate)}',
                            ),
                          ),
                        );
                      },
                    ),
                    const SizedBox(height: 24),
                  ],
                  Text('Installments', style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 4),
                  Text(
                    '${schedule.tenureMonths} months at ${_currency.format(schedule.summary.totalInstallments == 0 ? 0 : schedule.installments.first.amount)}/month',
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                  const SizedBox(height: 12),
                  PremiumCard(
                    padding: EdgeInsets.zero,
                    child: Column(
                      children: [
                        for (int i = 0; i < schedule.installments.length; i++) ...[
                          _InstallmentTile(
                            installment: schedule.installments[i],
                            isNextPayable: schedule.installments[i].id == nextPendingId,
                            paying: _paying,
                            onPayNow: () => _payNow(schedule.installments[i]),
                          ),
                          if (i != schedule.installments.length - 1) const Divider(height: 1, indent: 16),
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

final _currency = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);
final _dateFmt = DateFormat('d MMM yyyy');

class _ScheduleHeader extends StatelessWidget {
  const _ScheduleHeader({required this.schedule});

  final EmiSchedule schedule;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final summary = schedule.summary;
    final totalCount = summary.totalInstallments == 0 ? 1 : summary.totalInstallments;
    final progress = (summary.paidCount / totalCount).clamp(0.0, 1.0);

    return PremiumCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('NBFC-APP-${schedule.loanId}', style: theme.textTheme.titleMedium),
          Text(
            '${schedule.tenureMonths} months · ${schedule.interestRatePercent.toStringAsFixed(1)}% p.a.',
            style: theme.textTheme.bodySmall,
          ),
          const SizedBox(height: 16),
          ClipRRect(
            borderRadius: BorderRadius.circular(AppRadius.pill),
            child: LinearProgressIndicator(
              value: progress,
              minHeight: 8,
              backgroundColor: AppColors.primary.withValues(alpha: 0.1),
              valueColor: const AlwaysStoppedAnimation(AppColors.primary),
            ),
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              Expanded(
                child: _EmiStat(
                  label: 'Paid',
                  count: summary.paidCount,
                  amount: _currency.format(summary.totalPaid),
                  color: AppColors.success,
                  icon: Icons.check_circle_rounded,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _EmiStat(
                  label: 'Remaining',
                  count: summary.remainingCount,
                  amount: _currency.format(summary.totalRemaining),
                  color: AppColors.warning,
                  icon: Icons.pending_actions_rounded,
                ),
              ),
            ],
          ),
          if (summary.nextDueDate != null) ...[
            const SizedBox(height: 12),
            Row(
              children: [
                const Icon(Icons.event_rounded, size: 16, color: AppColors.textSecondaryLight),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    'Next due ${_dateFmt.format(summary.nextDueDate!)} · ${_currency.format(summary.nextDueAmount)}',
                    style: Theme.of(context).textTheme.bodySmall,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}

class _EmiStat extends StatelessWidget {
  const _EmiStat({required this.label, required this.count, required this.amount, required this.color, required this.icon});

  final String label;
  final int count;
  final String amount;
  final Color color;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppRadius.md),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(icon, size: 15, color: color),
              const SizedBox(width: 5),
              Text('$label · $count', style: theme.textTheme.labelSmall?.copyWith(color: color, fontWeight: FontWeight.w700)),
            ],
          ),
          const SizedBox(height: 6),
          Text(amount, style: theme.textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w800), maxLines: 1, overflow: TextOverflow.ellipsis),
        ],
      ),
    );
  }
}

class _InstallmentTile extends StatelessWidget {
  const _InstallmentTile({
    required this.installment,
    required this.isNextPayable,
    required this.paying,
    required this.onPayNow,
  });

  final EmiInstallment installment;
  final bool isNextPayable;
  final bool paying;
  final VoidCallback onPayNow;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final statusInfo = EmiStatusInfo.forStatus(installment.status);
    final isPaid = installment.status == 'paid';

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 32,
                height: 32,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: statusInfo.color.withValues(alpha: 0.12),
                  shape: BoxShape.circle,
                ),
                child: isPaid
                    ? const Icon(Icons.check_rounded, size: 16, color: AppColors.success)
                    : Text(
                        '${installment.installmentNumber}',
                        style: theme.textTheme.labelSmall?.copyWith(fontWeight: FontWeight.w700, color: statusInfo.color),
                      ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(_dateFmt.format(installment.dueDate), style: theme.textTheme.titleSmall),
                    Text(
                      'Principal ${_currency.format(installment.principalComponent)} · Interest ${_currency.format(installment.interestComponent)}',
                      style: theme.textTheme.labelSmall,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Text(_currency.format(installment.amount), style: theme.textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w700)),
                  Container(
                    margin: const EdgeInsets.only(top: 2),
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                    decoration: BoxDecoration(
                      color: statusInfo.color.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(AppRadius.pill),
                    ),
                    child: Text(
                      statusInfo.label,
                      style: theme.textTheme.labelSmall?.copyWith(color: statusInfo.color, fontWeight: FontWeight.w700),
                    ),
                  ),
                ],
              ),
            ],
          ),
          if (isNextPayable) ...[
            const SizedBox(height: 10),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                onPressed: paying ? null : onPayNow,
                child: paying
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                      )
                    : const Text('Pay Now'),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _EmptyState extends StatelessWidget {
  const _EmptyState({required this.onRefresh});

  final Future<void> Function() onRefresh;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return RefreshIndicator(
      onRefresh: onRefresh,
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        children: [
          SizedBox(
            height: MediaQuery.of(context).size.height * 0.6,
            child: Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.event_note_rounded, size: 56, color: AppColors.textSecondaryLight),
                  const SizedBox(height: 12),
                  Text('No EMI schedule yet for this loan', style: theme.textTheme.titleMedium),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.message, required this.onRetry});

  final String message;
  final Future<void> Function() onRetry;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline_rounded, size: 48, color: AppColors.error),
            const SizedBox(height: 12),
            Text(message, style: theme.textTheme.bodyMedium, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            OutlinedButton(onPressed: onRetry, child: const Text('Try Again')),
          ],
        ),
      ),
    );
  }
}
