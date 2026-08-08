import 'dart:async';
import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/widgets/premium_card.dart';
import '../../credit_score/widgets/credit_score_gauge.dart';

class CreditScoreCard extends StatefulWidget {
  final int score;
  final String band;
  final VoidCallback onTap;

  const CreditScoreCard({super.key, required this.score, required this.band, required this.onTap});

  @override
  State<CreditScoreCard> createState() => _CreditScoreCardState();
}

class _CreditScoreCardState extends State<CreditScoreCard> with SingleTickerProviderStateMixin {
  late final AnimationController _liveDotController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1000),
  )..repeat(reverse: true);

  late Timer _syncTimer;
  int _secondsAgo = 0;

  @override
  void initState() {
    super.initState();
    _syncTimer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted) setState(() => _secondsAgo++);
    });
  }

  @override
  void dispose() {
    _liveDotController.dispose();
    _syncTimer.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return PremiumCard(
      onTap: widget.onTap,
      child: Row(
        children: [
          SizedBox(
            width: 92,
            height: 60,
            child: CreditScoreGauge(score: widget.score, width: 92, height: 60, strokeWidth: 10, live: true),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text('Your Credit Health', style: theme.textTheme.titleSmall),
                    const SizedBox(width: 6),
                    AnimatedBuilder(
                      animation: _liveDotController,
                      builder: (context, _) => Container(
                        width: 6,
                        height: 6,
                        decoration: BoxDecoration(
                          color: AppColors.success.withValues(alpha: 0.4 + _liveDotController.value * 0.6),
                          shape: BoxShape.circle,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Row(
                  children: [
                    Text('${widget.score}', style: theme.textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w800)),
                    const SizedBox(width: 8),
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                      decoration: BoxDecoration(
                        color: AppColors.success.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(AppRadius.pill),
                      ),
                      child: Text(
                        widget.band,
                        style: theme.textTheme.labelSmall?.copyWith(color: AppColors.success, fontWeight: FontWeight.w700),
                      ),
                    ),
                  ],
                ),
                Text(
                  _secondsAgo < 60 ? 'CIBIL Score · synced ${_secondsAgo}s ago' : 'CIBIL Score · live monitoring',
                  style: theme.textTheme.bodySmall,
                ),
              ],
            ),
          ),
          const Icon(Icons.chevron_right_rounded, color: AppColors.textSecondaryLight),
        ],
      ),
    );
  }
}
