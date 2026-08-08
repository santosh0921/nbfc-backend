import 'package:flutter/material.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';

class _Stage {
  const _Stage(this.title, this.subtitle, this.icon);
  final String title;
  final String subtitle;
  final IconData icon;
}

/// Menu → Track Application: a real loan-application status tracker
/// (distinct from the application form itself). Shows a mock in-progress
/// application moving through underwriting stages with a live-animated
/// timeline — staggered entrance, a filling progress bar, a pulsing
/// "radar" ring on the active stage, and a blinking Live indicator.
class TrackApplicationScreen extends StatefulWidget {
  const TrackApplicationScreen({super.key});

  @override
  State<TrackApplicationScreen> createState() => _TrackApplicationScreenState();
}

class _TrackApplicationScreenState extends State<TrackApplicationScreen> with TickerProviderStateMixin {
  static const _stages = [
    _Stage('Application Submitted', 'Your details and documents were received.', Icons.description_rounded),
    _Stage('Document Verification', 'KYC and income documents are being verified.', Icons.fact_check_rounded),
    _Stage('Bank Details Verification', 'Your bank account and IFSC details are being validated.', Icons.account_balance_wallet_rounded),
    _Stage('Video KYC Call', 'Our employee will contact you for video KYC — keep your documentation ready.', Icons.video_call_rounded),
    _Stage('Credit Assessment', 'Your credit profile is under underwriting review.', Icons.insights_rounded),
    _Stage('Loan Approval', 'Final approval and sanction letter generation.', Icons.verified_rounded),
    _Stage('Agreement e-Signed', 'Your loan agreement has been signed and recorded.', Icons.draw_rounded),
    _Stage('Disbursement Initiated', 'Your loan amount is being processed for transfer.', Icons.sync_alt_rounded),
    _Stage('Funds Disbursed', 'Loan amount has been credited to your bank account.', Icons.account_balance_rounded),
  ];

  static const int _currentStage = 4;

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

  late final Animation<double> _progressAnim = Tween<double>(
    begin: 0,
    end: _currentStage / (_stages.length - 1),
  ).animate(CurvedAnimation(parent: _entranceController, curve: const Interval(0.2, 1.0, curve: Curves.easeOutCubic)));

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
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
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
                            Text('Personal Loan · ₹3,00,000', style: theme.textTheme.titleSmall),
                            const SizedBox(height: 4),
                            Text('Application ID: NBFC-APL-208451', style: theme.textTheme.bodySmall),
                          ],
                        ),
                      ),
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
                  const SizedBox(height: 18),
                  AnimatedBuilder(
                    animation: _progressAnim,
                    builder: (context, _) => Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            Text('Overall Progress', style: theme.textTheme.bodySmall),
                            Text(
                              '${(_progressAnim.value * 100).round()}%',
                              style: theme.textTheme.labelMedium?.copyWith(color: AppColors.primary, fontWeight: FontWeight.w800),
                            ),
                          ],
                        ),
                        const SizedBox(height: 8),
                        ClipRRect(
                          borderRadius: BorderRadius.circular(AppRadius.pill),
                          child: LinearProgressIndicator(
                            value: _progressAnim.value,
                            minHeight: 7,
                            backgroundColor: AppColors.primary.withValues(alpha: 0.1),
                            valueColor: const AlwaysStoppedAnimation(AppColors.primary),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 24),
            Text('Application Progress', style: theme.textTheme.titleMedium),
            const SizedBox(height: 16),
            for (int i = 0; i < _stages.length; i++)
              _AnimatedStageTile(
                entrance: _entranceController,
                pulse: _pulseController,
                index: i,
                total: _stages.length,
                stage: _stages[i],
                state: i < _currentStage
                    ? _StageState.done
                    : i == _currentStage
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
                      decoration: BoxDecoration(
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
