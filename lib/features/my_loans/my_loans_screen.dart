import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/loan_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/widgets/premium_card.dart';
import '../../models/loan_application.dart';
import 'widgets/loan_letter_actions.dart';

class MyLoansScreen extends StatefulWidget {
  const MyLoansScreen({super.key, this.initialLoanId, this.initialLetterType});

  /// When arriving via a "Sanction/Disbursement Letter ready" notification
  /// deep link, auto-opens this loan's detail sheet (which hosts
  /// [LoanLetterActionsSection]) as soon as the loan list has loaded,
  /// instead of requiring the customer to find and tap the card manually.
  final int? initialLoanId;

  /// Which letter row to visually highlight once the detail sheet opens,
  /// alongside [initialLoanId] — "Sanction Letter" or "Disbursement Letter".
  final String? initialLetterType;

  @override
  State<MyLoansScreen> createState() => _MyLoansScreenState();
}

class _MyLoansScreenState extends State<MyLoansScreen> {
  late Future<List<LoanApplication>> _future;
  bool _didDeepLink = false;

  @override
  void initState() {
    super.initState();
    _future = _load();
    if (widget.initialLoanId != null) {
      _future.then((_) => _openDeepLinkedLoan()).catchError((_) {});
    }
  }

  void _openDeepLinkedLoan() {
    if (_didDeepLink || !mounted || widget.initialLoanId == null) return;
    _didDeepLink = true;
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => _LoanDetailSheet(
        loanId: widget.initialLoanId!,
        highlightLetterType: widget.initialLetterType,
      ),
    );
  }

  Future<List<LoanApplication>> _load() async {
    final token = AuthProvider.instance.token;
    if (token == null) return [];
    return LoanApiService.mine(token);
  }

  Future<void> _refresh() async {
    setState(() => _future = _load());
    await _future;
  }

  Future<void> _requestTopUp(LoanApplication loan) async {
    final submitted = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => _TopUpSheet(loan: loan),
    );
    if (submitted != true || !mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text(
          'Top-up request submitted. It will go through the same verification and sanction process as a new application.',
        ),
      ),
    );
    await _refresh();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('My Loans')),
      body: SafeArea(
        child: FutureBuilder<List<LoanApplication>>(
          future: _future,
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError) {
              final message = snapshot.error is ApiException ? (snapshot.error as ApiException).message : 'Could not load your loans.';
              return _ErrorState(message: message, onRetry: _refresh);
            }
            final loans = snapshot.data ?? const [];
            if (loans.isEmpty) {
              return _EmptyState(onRefresh: _refresh);
            }
            return RefreshIndicator(
              onRefresh: _refresh,
              child: ListView.separated(
                padding: const EdgeInsets.fromLTRB(20, 8, 20, 40),
                itemCount: loans.length,
                separatorBuilder: (_, __) => const SizedBox(height: 14),
                itemBuilder: (context, i) => _LoanCard(
                  loan: loans[i],
                  onTopUpRequested: () => _requestTopUp(loans[i]),
                ),
              ),
            );
          },
        ),
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
                  const Icon(Icons.account_balance_rounded, size: 56, color: AppColors.textSecondaryLight),
                  const SizedBox(height: 12),
                  Text('No loan applications yet', style: theme.textTheme.titleMedium),
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

class _LoanCard extends StatelessWidget {
  const _LoanCard({required this.loan, required this.onTopUpRequested});

  final LoanApplication loan;
  final VoidCallback onTopUpRequested;

  Color _statusColor(String status) {
    switch (status) {
      case 'sanctioned':
      case 'disbursed':
        return AppColors.success;
      case 'rejected':
        return AppColors.error;
      case 'submitted':
      case 'auto_verified':
      case 'assigned':
      case 'verified':
        return AppColors.warning;
      default:
        return AppColors.textSecondaryLight;
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);
    final statusInfo = LoanStatusInfo.forStatus(loan.status);
    final color = _statusColor(loan.status);

    return PressableLoanCard(
      onTap: () => showModalBottomSheet(
        context: context,
        isScrollControlled: true,
        backgroundColor: Colors.transparent,
        builder: (_) => _LoanDetailSheet(loanId: loan.id),
      ),
      child: PremiumCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(loan.category, style: theme.textTheme.titleMedium, maxLines: 1, overflow: TextOverflow.ellipsis),
                      Text('NBFC-APP-${loan.id}', style: theme.textTheme.bodySmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                    ],
                  ),
                ),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(
                    color: color.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(999),
                  ),
                  child: Text(
                    statusInfo.label,
                    style: theme.textTheme.labelSmall?.copyWith(color: color, fontWeight: FontWeight.w700),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Text(statusInfo.description, style: theme.textTheme.bodySmall),
            const SizedBox(height: 16),
            Row(
              children: [
                _Stat(label: 'Requested', value: currency.format(loan.amountRequested)),
                if (loan.status == 'disbursed')
                  _Stat(label: 'Disbursed', value: currency.format(loan.disbursedAmount))
                else
                  _Stat(label: 'Applied', value: DateFormat('d MMM yyyy').format(loan.createdAt)),
              ],
            ),
            if (loan.status == 'disbursed') ...[
              const SizedBox(height: 12),
              SizedBox(
                width: double.infinity,
                child: OutlinedButton.icon(
                  onPressed: () => context.push('/emi-schedule', extra: '${loan.id}'),
                  icon: const Icon(Icons.event_note_rounded, size: 18),
                  label: const Text('View EMI Schedule'),
                ),
              ),
              const SizedBox(height: 8),
              SizedBox(
                width: double.infinity,
                child: OutlinedButton.icon(
                  onPressed: onTopUpRequested,
                  style: OutlinedButton.styleFrom(foregroundColor: AppColors.primary),
                  icon: const Icon(Icons.add_card_rounded, size: 18),
                  label: const Text('Request Top-Up'),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class PressableLoanCard extends StatelessWidget {
  const PressableLoanCard({super.key, required this.child, required this.onTap});

  final Widget child;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      borderRadius: BorderRadius.circular(16),
      onTap: onTap,
      child: child,
    );
  }
}

class _Stat extends StatelessWidget {
  final String label;
  final String value;

  const _Stat({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Expanded(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: theme.textTheme.labelSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
          const SizedBox(height: 2),
          Text(value, style: theme.textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
        ],
      ),
    );
  }
}

class _LoanDetailSheet extends StatefulWidget {
  const _LoanDetailSheet({required this.loanId, this.highlightLetterType});

  final int loanId;
  final String? highlightLetterType;

  @override
  State<_LoanDetailSheet> createState() => _LoanDetailSheetState();
}

class _LoanDetailSheetState extends State<_LoanDetailSheet> {
  late final Future<LoanApplication> _future = _load();

  Future<LoanApplication> _load() async {
    final token = AuthProvider.instance.token!;
    return LoanApiService.detail(token, widget.loanId);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);

    return DraggableScrollableSheet(
      initialChildSize: 0.6,
      maxChildSize: 0.9,
      minChildSize: 0.4,
      expand: false,
      builder: (context, scrollController) {
        return Container(
          decoration: BoxDecoration(
            color: Theme.of(context).scaffoldBackgroundColor,
            borderRadius: const BorderRadius.vertical(top: Radius.circular(24)),
          ),
          child: FutureBuilder<LoanApplication>(
            future: _future,
            builder: (context, snapshot) {
              if (snapshot.connectionState == ConnectionState.waiting) {
                return const Center(child: CircularProgressIndicator());
              }
              if (snapshot.hasError || !snapshot.hasData) {
                return Center(
                  child: Text(
                    snapshot.error is ApiException ? (snapshot.error as ApiException).message : 'Could not load loan details.',
                  ),
                );
              }
              final loan = snapshot.data!;
              final statusInfo = LoanStatusInfo.forStatus(loan.status);
              return ListView(
                controller: scrollController,
                padding: const EdgeInsets.fromLTRB(20, 16, 20, 32),
                children: [
                  Center(
                    child: Container(
                      width: 40,
                      height: 4,
                      decoration: BoxDecoration(color: AppColors.borderLight, borderRadius: BorderRadius.circular(2)),
                    ),
                  ),
                  const SizedBox(height: 20),
                  Text('NBFC-APP-${loan.id}', style: theme.textTheme.titleLarge),
                  const SizedBox(height: 4),
                  Text(loan.category, style: theme.textTheme.bodyMedium),
                  const SizedBox(height: 16),
                  PremiumCard(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(statusInfo.label, style: theme.textTheme.titleMedium),
                        const SizedBox(height: 4),
                        Text(statusInfo.description, style: theme.textTheme.bodySmall),
                      ],
                    ),
                  ),
                  const SizedBox(height: 16),
                  PremiumCard(
                    child: Column(
                      children: [
                        _DetailRow(label: 'Amount Requested', value: currency.format(loan.amountRequested)),
                        const Divider(height: 20),
                        _DetailRow(label: 'Purpose', value: loan.purpose.isEmpty ? '—' : loan.purpose),
                        const Divider(height: 20),
                        _DetailRow(label: 'City', value: loan.city.isEmpty ? '—' : loan.city),
                        const Divider(height: 20),
                        _DetailRow(label: 'Applied On', value: DateFormat('d MMM yyyy, h:mm a').format(loan.createdAt)),
                      ],
                    ),
                  ),
                  if (loan.decisionRemarks != null && loan.decisionRemarks!.isNotEmpty) ...[
                    const SizedBox(height: 16),
                    PremiumCard(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            loan.status == 'rejected' ? 'Rejection Reason' : 'Decision Remarks',
                            style: theme.textTheme.titleSmall,
                          ),
                          const SizedBox(height: 6),
                          Text(loan.decisionRemarks!, style: theme.textTheme.bodySmall),
                        ],
                      ),
                    ),
                  ],
                  if (loan.status == 'disbursed') ...[
                    const SizedBox(height: 16),
                    PremiumCard(
                      child: Column(
                        children: [
                          _DetailRow(label: 'Disbursed Amount', value: currency.format(loan.disbursedAmount)),
                          if (loan.disbursedAt != null) ...[
                            const Divider(height: 20),
                            _DetailRow(label: 'Disbursed On', value: DateFormat('d MMM yyyy, h:mm a').format(loan.disbursedAt!)),
                          ],
                        ],
                      ),
                    ),
                  ],
                  if (loan.status == 'sanctioned' || loan.status == 'disbursed') ...[
                    const SizedBox(height: 16),
                    LoanLetterActionsSection(loan: loan, highlightLetterType: widget.highlightLetterType),
                  ],
                  if (loan.verificationReport != null && loan.verificationReport!.isNotEmpty) ...[
                    const SizedBox(height: 16),
                    PremiumCard(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text('Verification Report', style: theme.textTheme.titleSmall),
                          const SizedBox(height: 6),
                          Text(loan.verificationReport!, style: theme.textTheme.bodySmall),
                        ],
                      ),
                    ),
                  ],
                ],
              );
            },
          ),
        );
      },
    );
  }
}

class _DetailRow extends StatelessWidget {
  const _DetailRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label, style: theme.textTheme.bodyMedium),
        Flexible(child: Text(value, style: theme.textTheme.titleSmall, textAlign: TextAlign.right)),
      ],
    );
  }
}

/// Bottom sheet form for `POST /auth/loans/:id/topup` — collects the
/// requested top-up amount and purpose, then submits. Pops `true` on
/// success so the caller can refresh the loan list and show a confirmation.
class _TopUpSheet extends StatefulWidget {
  const _TopUpSheet({required this.loan});

  final LoanApplication loan;

  @override
  State<_TopUpSheet> createState() => _TopUpSheetState();
}

class _TopUpSheetState extends State<_TopUpSheet> {
  final _formKey = GlobalKey<FormState>();
  final _amountController = TextEditingController();
  final _purposeController = TextEditingController();
  bool _submitting = false;
  String? _error;

  @override
  void dispose() {
    _amountController.dispose();
    _purposeController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final formValid = _formKey.currentState?.validate() ?? false;
    if (!formValid) return;
    final token = AuthProvider.instance.token;
    if (token == null) return;
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      await LoanApiService.topUp(
        token,
        widget.loan.id,
        amount: double.parse(_amountController.text.trim()),
        purpose: _purposeController.text.trim(),
      );
      if (mounted) Navigator.of(context).pop(true);
    } on ApiException catch (e) {
      setState(() {
        _submitting = false;
        _error = e.message;
      });
    } catch (_) {
      setState(() {
        _submitting = false;
        _error = 'Something went wrong. Please try again.';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
      child: Container(
        decoration: BoxDecoration(
          color: Theme.of(context).scaffoldBackgroundColor,
          borderRadius: const BorderRadius.vertical(top: Radius.circular(24)),
        ),
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(20, 16, 20, 32),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Center(
                  child: Container(
                    width: 40,
                    height: 4,
                    decoration: BoxDecoration(color: AppColors.borderLight, borderRadius: BorderRadius.circular(2)),
                  ),
                ),
                const SizedBox(height: 20),
                Text('Request Top-Up', style: theme.textTheme.titleLarge),
                const SizedBox(height: 4),
                Text(
                  'On NBFC-APP-${widget.loan.id} · ${widget.loan.category}',
                  style: theme.textTheme.bodySmall,
                ),
                const SizedBox(height: 20),
                Text('Amount', style: theme.textTheme.titleSmall),
                const SizedBox(height: 8),
                TextFormField(
                  controller: _amountController,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: const InputDecoration(
                    prefixText: '₹ ',
                    hintText: 'e.g. 50000',
                    border: OutlineInputBorder(),
                  ),
                  validator: (v) {
                    final trimmed = v?.trim() ?? '';
                    if (trimmed.isEmpty) return 'Enter an amount';
                    final parsed = double.tryParse(trimmed);
                    if (parsed == null || parsed <= 0) return 'Enter a valid amount';
                    return null;
                  },
                ),
                const SizedBox(height: 16),
                Text('Purpose', style: theme.textTheme.titleSmall),
                const SizedBox(height: 8),
                TextFormField(
                  controller: _purposeController,
                  maxLines: 3,
                  decoration: const InputDecoration(
                    hintText: 'What is this top-up for?',
                    border: OutlineInputBorder(),
                  ),
                  validator: (v) => (v == null || v.trim().isEmpty) ? 'Please describe the purpose' : null,
                ),
                if (_error != null) ...[
                  const SizedBox(height: 12),
                  Text(_error!, style: theme.textTheme.bodySmall?.copyWith(color: AppColors.error)),
                ],
                const SizedBox(height: 20),
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton(
                    onPressed: _submitting ? null : _submit,
                    style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(50)),
                    child: _submitting
                        ? const SizedBox(
                            height: 20,
                            width: 20,
                            child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                          )
                        : const Text('Submit Top-Up Request'),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
