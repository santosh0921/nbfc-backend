import 'package:flutter/material.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/constants/app_spacing.dart';

/// Animated KPI tile for the Recovery dashboard — counts up from zero on
/// first build, matching the enterprise-fintech feel of the rest of the
/// module without introducing a new design primitive.
class RecoveryKpiCard extends StatefulWidget {
  const RecoveryKpiCard({
    super.key,
    required this.label,
    required this.value,
    required this.icon,
    required this.color,
    this.onTap,
    this.isCurrency = false,
  });

  final String label;
  final double value;
  final IconData icon;
  final Color color;
  final VoidCallback? onTap;
  final bool isCurrency;

  @override
  State<RecoveryKpiCard> createState() => _RecoveryKpiCardState();
}

class _RecoveryKpiCardState extends State<RecoveryKpiCard> with SingleTickerProviderStateMixin {
  late final AnimationController _controller =
      AnimationController(vsync: this, duration: const Duration(milliseconds: 900))..forward();
  late final Animation<double> _animation = CurvedAnimation(parent: _controller, curve: Curves.easeOutCubic);

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  String _format(double v) {
    if (widget.isCurrency) {
      if (v >= 100000) return '₹${(v / 100000).toStringAsFixed(1)}L';
      if (v >= 1000) return '₹${(v / 1000).toStringAsFixed(1)}K';
      return '₹${v.toStringAsFixed(0)}';
    }
    return v.toStringAsFixed(0);
  }

  @override
  Widget build(BuildContext context) {
    return AppCard(
      onTap: widget.onTap,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 36,
            height: 36,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: widget.color.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(AppSpacing.radiusMd),
            ),
            child: Icon(widget.icon, color: widget.color, size: 18),
          ),
          const SizedBox(height: AppSpacing.sm),
          AnimatedBuilder(
            animation: _animation,
            builder: (context, _) => Text(
              _format(widget.value * _animation.value),
              style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w800),
            ),
          ),
          const SizedBox(height: 2),
          Text(
            widget.label,
            style: Theme.of(context).textTheme.bodySmall,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
          ),
        ],
      ),
    );
  }
}
