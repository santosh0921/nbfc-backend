import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/loan_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../../models/loan_application.dart';

class _Stage {
  const _Stage(this.title, this.subtitle, this.icon);
  final String title;
  final String subtitle;
  final IconData icon;
}

// Honest, reduced set of stages matching what the backend actually
// tracks (internal/models/loan.go's status vocabulary) — the previous
// version invented UI-only stages ("Bank Details Verification", "Video
// KYC Call", "Agreement e-Signed") that don't correspond to any real
// status transition, so there was no way to ever compute where a real
// loan actually sat on that timeline.
const _stages = [
  _Stage('Application Submitted', 'Your details and documents were received.', Icons.description_rounded),
  _Stage('Verification', 'Being auto-verified or reviewed by a field officer.', Icons.fact_check_rounded),
  _Stage('Credit Assessment & Sanction', 'Underwriting review and sanction decision.', Icons.insights_rounded),
  _Stage('Disbursement', 'Loan amount transferred to your bank account.', Icons.account_balance_rounded),
];

/// Maps a real backend `LoanApplication.status` string onto an index into
/// [_stages] — null return means "rejected", which is shown as a distinct
/// terminal state rather than forced onto the linear progress track.
int? _stageIndexFor(String status) => switch (status) {
      'submitted' => 0,
      'assigned' || 'verified' || 'auto_verified' => 1,
      'sanctioned' => 2,
      'disbursed' => 3,
      'rejected' => null,
      _ => 0,
    };

/// Menu → Track Application: a real loan-application status tracker
/// driven by the customer's actual loan data (`GET /auth/loans/mine`),
/// not a fixed mock stage. If the customer has more than one loan, the
/// most recent one still in progress is tracked by default, with a
/// selector to switch between all of them.
class TrackApplicationScreen extends StatefulWidget {
  const TrackApplicationScreen({super.key});

  @override
  State<TrackApplicationScreen> createState() => _TrackApplicationScreenState();
}

class _TrackApplicationScreenState extends State<TrackApplicationScreen> with TickerProviderStateMixin {
  late Future<List<LoanApplication>> _future;
  LoanApplication? _selected;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  Future<List<LoanApplication>> _load() async {
    final token = AuthProvider.instance.token;
    if (token == null) return const [];
    final loans = await LoanApiService.mine(token);
    // Default to the most recently applied loan that's still actively in
    // progress (not yet disbursed or rejected); fall back to the single
    // most recent loan overall if every one of them is already final.
    final inProgress = loans.where((l) => l.status != 'disbursed' && l.status != 'rejected').toList()
      ..sort((a, b) => b.createdAt.compareTo(a.createdAt));
    final sorted = List<LoanApplication>.from(loans)..sort((a, b) => b.createdAt.compareTo(a.createdAt));
    _selected = inProgress.isNotEmpty ? inProgress.first : (sorted.isNotEmpty ? sorted.first : null);
    return sorted;
  }

  late final AnimationController _entranceController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1100),
  )..forward();

  late final AnimationController _pulseController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1500),
  )..repeat();

  late final AnimationController _liveDotController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1000),
  )..repeat(reverse: true);

  @override
  void dispose() {
    _entranceController.dispose();
    _pulseController.dispose();
    _liveDotController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Track Application')),
      body: SafeArea(
        child: FutureBuilder<List<LoanApplication>>(
          future: _future,
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError) {
              final message = snapshot.error is ApiException ? (snapshot.error as ApiException).message : 'Could not load your applications.';
              return Center(child: Padding(padding: const EdgeInsets.all(24), child: Text(message, textAlign: TextAlign.center)));
            }
            final loans = snapshot.data ?? const [];
            if (loans.isEmpty || _selected == null) {
              return Center(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Icon(Icons.description_outlined, size: 48, color: AppColors.textSecondaryLight),
                      const SizedBox(height: 12),
                      Text('No loan applications yet.', style: theme.textTheme.titleMedium),
                    ],
                  ),
                ),
              );
            }

            final loan = _selected!;
            final stageIndex = _stageIndexFor(loan.status);
            final currency = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);

            return ListView(
              padding: const EdgeInsets.all(20),
              children: [
                if (loans.length > 1) ...[
                  DropdownButtonFormField<LoanApplication>(
                    initialValue: loan,
                    decoration: const InputDecoration(labelText: 'Tracking', border: OutlineInputBorder()),
                    items: [
                      for (final l in loans)
                        DropdownMenuItem(value: l, child: Text('${l.category} · NBFC-APP-${l.id}', overflow: TextOverflow.ellipsis)),
                    ],
                    onChanged: (v) => setState(() => _selected = v),
                  ),
                  const SizedBox(height: 16),
                ],
                PremiumCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Container(
                            width: 48,
                            height: 48,
                            decoration: BoxDecoration(
                              color: AppColors.primary.withValues(alpha: 0.1),
                              borderRadius: BorderRadius.circular(AppRadius.sm),
                            ),
                            child: const Icon(Icons.request_quote_rounded, color: AppColors.primary),
                          ),
                          const SizedBox(width: 14),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text('${loan.category} · ${currency.format(loan.amountRequested)}', style: theme.textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                                const SizedBox(height: 4),
                                Text('Application ID: NBFC-APP-${loan.id}', style: theme.textTheme.bodySmall),
                              ],
                            ),
                          ),
                          if (stageIndex != null)
                            AnimatedBuilder(
                              animation: _liveDotController,
                              builder: (context, _) => Container(
                                padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
                                decoration: BoxDecoration(
                                  color: AppColors.success.withValues(alpha: 0.12),
                                  borderRadius: BorderRadius.circular(AppRadius.pill),
                                ),
                                child: Row(
                                  mainAxisSize: MainAxisSize.min,
                                  children: [
                                    Container(
                                      width: 7,
                                      height: 7,
                                      decoration: BoxDecoration(
                                        color: AppColors.success.withValues(alpha: 0.4 + _liveDotController.value * 0.6),
                                        shape: BoxShape.circle,
                                      ),
                                    ),
                                    const SizedBox(width: 5),
                                    Text('LIVE', style: theme.textTheme.labelSmall?.copyWith(color: AppColors.success, fontWeight: FontWeight.w800, letterSpacing: 0.4)),
                                  ],
                                ),
                              ),
                            ),
                        ],
                      ),
                      if (stageIndex != null) ...[
                        const SizedBox(height: 18),
                        Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            Text('Overall Progress', style: theme.textTheme.bodySmall),
                            Text(
                              '${(((stageIndex + 1) / _stages.length) * 100).round()}%',
                              style: theme.textTheme.labelMedium?.copyWith(color: AppColors.primary, fontWeight: FontWeight.w800),
                            ),
                          ],
                        ),
                        const SizedBox(height: 8),
                        ClipRRect(
                          borderRadius: BorderRadius.circular(AppRadius.pill),
                          child: LinearProgressIndicator(
                            value: (stageIndex + 1) / _stages.length,
                            minHeight: 7,
                            backgroundColor: AppColors.primary.withValues(alpha: 0.1),
                            valueColor: const AlwaysStoppedAnimation(AppColors.primary),
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
                const SizedBox(height: 24),
                if (stageIndex == null) ...[
                  PremiumCard(
                    child: Row(
                      children: [
                        const Icon(Icons.cancel_rounded, color: AppColors.error),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text('Application Rejected', style: theme.textTheme.titleSmall?.copyWith(color: AppColors.error)),
                              if (loan.decisionRemarks != null && loan.decisionRemarks!.isNotEmpty) ...[
                                const SizedBox(height: 4),
                                Text(loan.decisionRemarks!, style: theme.textTheme.bodySmall),
                              ],
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                ] else ...[
                  Text('Application Progress', style: theme.textTheme.titleMedium),
                  const SizedBox(height: 16),
                  for (int i = 0; i < _stages.length; i++)
                    _AnimatedStageTile(
                      entrance: _entranceController,
                      pulse: _pulseController,
                      index: i,
                      total: _stages.length,
                      stage: _stages[i],
                      state: i < stageIndex
                          ? _StageState.done
                          : i == stageIndex
                              ? _StageState.active
                              : _StageState.pending,
                      isLast: i == _stages.length - 1,
                    ),
                  const SizedBox(height: 8),
                  Container(
                    padding: const EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: AppColors.primary.withValues(alpha: 0.06),
                      borderRadius: BorderRadius.circular(AppRadius.lg),
                    ),
                    child: Row(
                      children: [
                        const Icon(Icons.info_outline_rounded, color: AppColors.primary, size: 20),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Text(
                            'Credit assessment usually takes 1-2 business days. You\'ll be notified the moment your status changes.',
                            style: theme.textTheme.bodySmall,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ],
            );
          },
        ),
      ),
    );
  }
}

enum _StageState { done, active, pending }

class _AnimatedStageTile extends StatelessWidget {
  const _AnimatedStageTile({
    required this.entrance,
    required this.pulse,
    required this.index,
    required this.total,
    required this.stage,
    required this.state,
    required this.isLast,
  });

  final AnimationController entrance;
  final AnimationController pulse;
  final int index;
  final int total;
  final _Stage stage;
  final _StageState state;
  final bool isLast;

  @override
  Widget build(BuildContext context) {
    final start = (index / total * 0.6).clamp(0.0, 0.85);
    final end = (start + 0.35).clamp(0.0, 1.0);
    final fade = CurvedAnimation(parent: entrance, curve: Interval(start, end, curve: Curves.easeOutCubic));

    return AnimatedBuilder(
      animation: fade,
      builder: (context, child) => Opacity(
        opacity: fade.value.clamp(0.0, 1.0),
        child: Transform.translate(offset: Offset(0, (1 - fade.value.clamp(0.0, 1.0)) * 14), child: child),
      ),
      child: _StageTile(stage: stage, state: state, isLast: isLast, pulse: pulse),
    );
  }
}

class _StageTile extends StatelessWidget {
  const _StageTile({required this.stage, required this.state, required this.isLast, required this.pulse});

  final _Stage stage;
  final _StageState state;
  final bool isLast;
  final AnimationController pulse;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = switch (state) {
      _StageState.done => AppColors.success,
      _StageState.active => AppColors.primary,
      _StageState.pending => AppColors.textSecondaryLight,
    };
    return IntrinsicHeight(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Column(
            children: [
              SizedBox(
                width: 44,
                height: 44,
                child: Stack(
                  alignment: Alignment.center,
                  children: [
                    if (state == _StageState.active)
                      AnimatedBuilder(
                        animation: pulse,
                        builder: (context, _) {
                          final t = pulse.value;
                          return Container(
                            width: 34 + t * 18,
                            height: 34 + t * 18,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              border: Border.all(color: AppColors.primary.withValues(alpha: (1 - t) * 0.5), width: 1.6),
                            ),
                          );
                        },
                      ),
                    Container(
                      width: 34,
                      height: 34,
                      decoration: BoxDecoration(
                        color: state == _StageState.pending ? Colors.transparent : color.withValues(alpha: 0.12),
                        shape: BoxShape.circle,
                        border: Border.all(color: color, width: state == _StageState.pending ? 1.5 : 0),
                      ),
                      child: Icon(
                        state == _StageState.done ? Icons.check_rounded : stage.icon,
                        size: 17,
                        color: color,
                      ),
                    ),
                  ],
                ),
              ),
              if (!isLast)
                Expanded(
                  child: TweenAnimationBuilder<double>(
                    tween: Tween(begin: 0, end: state == _StageState.done ? 1 : 0),
                    duration: const Duration(milliseconds: 500),
                    curve: Curves.easeOut,
                    builder: (context, value, _) => Container(
                      width: 2,
                      margin: const EdgeInsets.symmetric(vertical: 2),
                      decoration: const BoxDecoration(
                        color: AppColors.borderLight,
                      ),
                      child: FractionallySizedBox(
                        alignment: Alignment.topCenter,
                        heightFactor: value,
                        child: Container(color: AppColors.success.withValues(alpha: 0.6)),
                      ),
                    ),
                  ),
                ),
            ],
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Padding(
              padding: EdgeInsets.only(bottom: isLast ? 0 : 22, top: 4),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    stage.title,
                    style: theme.textTheme.titleSmall?.copyWith(
                      color: state == _StageState.pending ? AppColors.textSecondaryLight : null,
                      fontWeight: state == _StageState.active ? FontWeight.w700 : FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 3),
                  Text(stage.subtitle, style: theme.textTheme.bodySmall),
                  if (state == _StageState.active) ...[
                    const SizedBox(height: 6),
                    AnimatedBuilder(
                      animation: pulse,
                      builder: (context, _) => Container(
                        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                        decoration: BoxDecoration(
                          color: AppColors.primary.withValues(alpha: 0.08 + pulse.value * 0.08),
                          borderRadius: BorderRadius.circular(AppRadius.pill),
                        ),
                        child: Text('In Progress', style: theme.textTheme.labelSmall?.copyWith(color: AppColors.primary, fontWeight: FontWeight.w700)),
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
