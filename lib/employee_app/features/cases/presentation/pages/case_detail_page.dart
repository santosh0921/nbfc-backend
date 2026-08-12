import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/gradient_header.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/di/service_locator.dart';
import '../../../../core/config/app_config.dart';
import '../../../../core/storage/secure_storage_service.dart';
import '../../../../core/utils/demo_call.dart';
import '../../../../core/widgets/app_badge.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../auth/presentation/providers/auth_provider.dart' as employee_auth;
import '../../../visit_verification/presentation/pages/verification_flow_page.dart';
import '../../domain/entities/case_entity.dart';
import '../../domain/repositories/cases_repository.dart';
import '../../../../../core/services/call_service.dart';
import '../../../../../core/widgets/call_screen.dart';

class CaseDetailPage extends StatelessWidget {
  const CaseDetailPage({super.key, required this.caseId});

  final String caseId;

  @override
  Widget build(BuildContext context) {
    final repository = context.read<CasesRepository>();

    return Scaffold(
      body: FutureBuilder(
        future: Future.wait([repository.fetchCaseById(caseId), repository.fetchTimeline(caseId)]),
        builder: (context, snapshot) {
          if (!snapshot.hasData) {
            return const Center(child: CircularProgressIndicator());
          }
          final caseItem = snapshot.data![0] as CaseEntity;
          final timeline = snapshot.data![1] as List<CaseTimelineEvent>;

          return CustomScrollView(
            slivers: [
              SliverToBoxAdapter(
                child: GradientHeader(
                  title: caseItem.customerName,
                  subtitle: '${caseItem.caseNumber} · ${caseItem.loanType.label}',
                  trailing: IconButton(
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.arrow_back, color: AppColors.secondary),
                  ),
                ),
              ),
              SliverPadding(
                padding: const EdgeInsets.all(AppSpacing.md),
                sliver: SliverList(
                  delegate: SliverChildListDelegate([
                    _CustomerDetailsCard(caseItem: caseItem),
                    const SizedBox(height: AppSpacing.md),
                    _ReferencesCard(references: caseItem.references),
                    const SizedBox(height: AppSpacing.md),
                    _VerificationChecklistCard(loanType: caseItem.loanType),
                    const SizedBox(height: AppSpacing.md),
                    _TimelineCard(events: timeline),
                    const SizedBox(height: AppSpacing.xxl),
                  ]),
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _CustomerDetailsCard extends StatelessWidget {
  const _CustomerDetailsCard({required this.caseItem});

  final CaseEntity caseItem;

  @override
  Widget build(BuildContext context) {
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Flexible(
                child: Text(
                  'Customer Details',
                  style: Theme.of(context).textTheme.titleMedium,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const Spacer(),
              AppBadge(
                label: caseItem.status.label,
                variant: switch (caseItem.status) {
                  CaseStatus.assigned => AppBadgeVariant.info,
                  CaseStatus.inProgress => AppBadgeVariant.warning,
                  CaseStatus.pendingReview => AppBadgeVariant.gold,
                  CaseStatus.completed => AppBadgeVariant.success,
                  CaseStatus.rejected => AppBadgeVariant.danger,
                },
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          _DetailRow(icon: Icons.location_on_outlined, label: 'Address', value: caseItem.address),
          _DetailRow(icon: Icons.phone_outlined, label: 'Phone', value: caseItem.customerPhone),
          _DetailRow(icon: Icons.currency_rupee, label: 'Loan Amount', value: '₹${caseItem.loanAmount.toStringAsFixed(0)}'),
          if (caseItem.cibilScore != null)
            _DetailRow(icon: Icons.speed_outlined, label: 'CIBIL Score', value: '${caseItem.cibilScore}'),
          _DetailRow(
            icon: Icons.event_outlined,
            label: 'Due Date',
            value: '${caseItem.dueDate.day}/${caseItem.dueDate.month}/${caseItem.dueDate.year}',
          ),
          if (caseItem.timingPreference != null)
            _DetailRow(icon: Icons.schedule_outlined, label: 'Preferred Call Time', value: caseItem.timingPreference!),
          if (caseItem.visitDate != null)
            _DetailRow(
              icon: Icons.home_work_outlined,
              label: 'Property Visit',
              value: '${caseItem.visitDate!.day}/${caseItem.visitDate!.month}/${caseItem.visitDate!.year}'
                  '${caseItem.visitTimeSlot != null ? ', ${caseItem.visitTimeSlot}' : ''}',
            ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Expanded(
                child: AppButton(
                  label: 'Call Customer',
                  icon: Icons.call_outlined,
                  onPressed: () => _callAndConfirm(context, caseItem),
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: AppButton(
                  label: 'Navigate',
                  icon: Icons.navigation_outlined,
                  variant: AppButtonVariant.outline,
                  onPressed: () => launchUrl(
                    Uri.parse('https://www.google.com/maps/search/?api=1&query=${Uri.encodeComponent(caseItem.address)}'),
                    mode: LaunchMode.externalApplication,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          AppButton(
            label: 'Start Video Call',
            icon: Icons.videocam_outlined,
            variant: AppButtonVariant.outline,
            onPressed: () => _startVideoCall(context, caseItem),
          ),
        ],
      ),
    );
  }
}

/// Always shown after every "Call Customer" tap — the field officer must
/// log the customer's response before moving on, so the visit outcome is
/// never assumed silently.
Future<void> _callAndConfirm(BuildContext context, CaseEntity caseItem) async {
  await showCallOutcomeFlow(
    context,
    name: caseItem.customerName,
    phone: caseItem.customerPhone,
    onProceed: () => Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => VerificationFlowPage(
          caseId: caseItem.id,
          customerName: caseItem.customerName,
          loanType: caseItem.loanType.label,
          documentChecklist: caseItem.loanType.documentChecklist,
          customerPhone: caseItem.customerPhone,
          timingPreference: caseItem.timingPreference,
        ),
      ),
    ),
  );
}

/// Documents the officer captures as screenshots DURING/after the video
/// call itself — distinct from the generic loan-category document
/// checklist ("Aadhaar Card", "Salary Slip", ...) captured earlier in the
/// visit flow.
const _postCallScreenshotChecklist = ['Customer Photo', 'Aadhaar Card SS', 'PAN Card SS', 'Customer Signature SS'];

Future<void> _startVideoCall(BuildContext context, CaseEntity caseItem) async {
  final employeeName = context.read<employee_auth.AuthProvider>().employee?.name ?? 'Field Officer';
  final token = await ServiceLocator.instance.get<SecureStorageService>().read('auth_token');
  if (token == null || !context.mounted) return;
  final callService = CallService(wsBaseUrl: AppConfig.current.apiBaseUrl);
  await Navigator.of(context).push(
    MaterialPageRoute(
      builder: (_) => CallScreen(
        callService: callService,
        token: token,
        loanId: caseItem.id,
        isCaller: true,
        localDisplayName: employeeName,
        peerLabel: caseItem.customerName,
      ),
    ),
  );
  if (!context.mounted || !callService.wasEverActive) return;
  await Navigator.of(context).push(
    MaterialPageRoute(
      builder: (_) => VerificationFlowPage(
        caseId: caseItem.id,
        customerName: caseItem.customerName,
        loanType: caseItem.loanType.label,
        documentChecklist: _postCallScreenshotChecklist,
        customerPhone: caseItem.customerPhone,
        timingPreference: caseItem.timingPreference,
      ),
    ),
  );
}

/// References the customer submitted with the application (mandatory, 2 per
/// application — internal/loans/reference.go). Verification officers are
/// expected to call/cross-check these as part of field due diligence.
class _ReferencesCard extends StatelessWidget {
  const _ReferencesCard({required this.references});

  final List<LoanReference> references;

  @override
  Widget build(BuildContext context) {
    if (references.isEmpty) return const SizedBox.shrink();
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('References', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 4),
          Text(
            'Cross-check with the applicant\'s references',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
          ),
          const SizedBox(height: AppSpacing.sm),
          for (int i = 0; i < references.length; i++) ...[
            if (i != 0) Divider(height: AppSpacing.lg, color: AppColors.divider),
            _ReferenceTile(reference: references[i]),
          ],
        ],
      ),
    );
  }
}

class _ReferenceTile extends StatelessWidget {
  const _ReferenceTile({required this.reference});

  final LoanReference reference;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    reference.name,
                    style: Theme.of(context).textTheme.titleSmall,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  Text(
                    reference.relation,
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
            IconButton(
              onPressed: () => showDemoCall(context, name: reference.name, phone: reference.phone),
              icon: const Icon(Icons.call_outlined, color: AppColors.primary),
              tooltip: 'Call ${reference.name}',
            ),
          ],
        ),
        const SizedBox(height: 4),
        _DetailRow(icon: Icons.phone_outlined, label: 'Phone', value: reference.phone),
        _DetailRow(icon: Icons.badge_outlined, label: 'Aadhaar', value: reference.aadhaarMasked),
        _DetailRow(icon: Icons.credit_card_outlined, label: 'PAN', value: reference.panMasked),
      ],
    );
  }
}

class _DetailRow extends StatelessWidget {
  const _DetailRow({required this.icon, required this.label, required this.value});

  final IconData icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 18, color: AppColors.textSecondary),
          const SizedBox(width: AppSpacing.sm),
          SizedBox(width: 96, child: Text(label, style: Theme.of(context).textTheme.bodySmall)),
          Expanded(child: Text(value, style: Theme.of(context).textTheme.bodyMedium)),
        ],
      ),
    );
  }
}

class _VerificationChecklistCard extends StatelessWidget {
  const _VerificationChecklistCard({required this.loanType});

  final LoanType loanType;

  @override
  Widget build(BuildContext context) {
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Verification Checklist', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 4),
          Text(
            'Steps required for ${loanType.label}',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
          ),
          const SizedBox(height: AppSpacing.sm),
          ...loanType.verificationSteps.map((step) => Padding(
                padding: const EdgeInsets.symmetric(vertical: 6),
                child: Row(
                  children: [
                    const Icon(Icons.radio_button_unchecked, size: 20, color: AppColors.primary),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(child: Text(step, style: Theme.of(context).textTheme.bodyMedium)),
                  ],
                ),
              )),
        ],
      ),
    );
  }
}

class _TimelineCard extends StatelessWidget {
  const _TimelineCard({required this.events});

  final List<CaseTimelineEvent> events;

  @override
  Widget build(BuildContext context) {
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Case Timeline', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: AppSpacing.sm),
          for (int i = 0; i < events.length; i++)
            IntrinsicHeight(
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Column(
                    children: [
                      Container(
                        width: 10,
                        height: 10,
                        decoration: BoxDecoration(
                          color: i == events.length - 1 ? AppColors.gold : AppColors.primary,
                          shape: BoxShape.circle,
                        ),
                      ),
                      if (i != events.length - 1) Expanded(child: Container(width: 2, color: AppColors.divider)),
                    ],
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Padding(
                      padding: const EdgeInsets.only(bottom: AppSpacing.md),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(events[i].title, style: Theme.of(context).textTheme.titleSmall),
                          const SizedBox(height: 2),
                          Text(events[i].description, style: Theme.of(context).textTheme.bodySmall),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
