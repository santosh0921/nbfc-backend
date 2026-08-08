import '../../../../core/widgets/app_badge.dart';
import '../../domain/entities/recovery_case_entity.dart';

extension RecoveryPriorityBadgeX on RecoveryPriority {
  AppBadgeVariant get badgeVariant => switch (this) {
        RecoveryPriority.high => AppBadgeVariant.danger,
        RecoveryPriority.medium => AppBadgeVariant.warning,
        RecoveryPriority.low => AppBadgeVariant.info,
      };
}

extension RiskLevelBadgeX on RiskLevel {
  AppBadgeVariant get badgeVariant => switch (this) {
        RiskLevel.low => AppBadgeVariant.success,
        RiskLevel.medium => AppBadgeVariant.info,
        RiskLevel.high => AppBadgeVariant.warning,
        RiskLevel.critical => AppBadgeVariant.danger,
      };
}

extension RecoveryStatusBadgeX on RecoveryStatus {
  AppBadgeVariant get badgeVariant => switch (this) {
        RecoveryStatus.assigned => AppBadgeVariant.info,
        RecoveryStatus.contacted => AppBadgeVariant.info,
        RecoveryStatus.visitScheduled => AppBadgeVariant.warning,
        RecoveryStatus.visited => AppBadgeVariant.warning,
        RecoveryStatus.promiseToPay => AppBadgeVariant.gold,
        RecoveryStatus.partialPayment => AppBadgeVariant.gold,
        RecoveryStatus.fullPayment => AppBadgeVariant.success,
        RecoveryStatus.settlement => AppBadgeVariant.info,
        RecoveryStatus.escalated => AppBadgeVariant.danger,
        RecoveryStatus.legalRecommended => AppBadgeVariant.danger,
        RecoveryStatus.unableToLocate => AppBadgeVariant.neutral,
        RecoveryStatus.fraudSuspicion => AppBadgeVariant.danger,
        RecoveryStatus.closed => AppBadgeVariant.success,
      };
}
