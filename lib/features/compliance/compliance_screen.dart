import 'package:flutter/material.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../../mock/mock_data.dart';

/// RBI-mandated disclosures and legal/compliance information, all real
/// (static) content — Key Facts Statement basis, APR/fee disclosures per
/// loan product, Fair Practices Code, Grievance Redressal, and privacy/
/// terms summaries. Frontend-only informational screen.
class ComplianceScreen extends StatelessWidget {
  const ComplianceScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Legal & Compliance')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
          children: [
            PremiumCard(
              child: Row(
                children: [
                  const Icon(Icons.verified_user_rounded, color: AppColors.primary, size: 32),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('RBI-Registered NBFC', style: theme.textTheme.titleSmall),
                        Text('CIN: U65999MH2015PTC123456 · RBI Reg. No: N-13.02345', style: theme.textTheme.bodySmall),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 24),
            Text('Interest Rates & Charges by Product', style: theme.textTheme.titleMedium),
            const SizedBox(height: 4),
            Text('As disclosed in the Key Facts Statement for each product.', style: theme.textTheme.bodySmall),
            const SizedBox(height: 12),
            PremiumCard(
              padding: EdgeInsets.zero,
              child: Column(
                children: [
                  for (int i = 0; i < MockData.loanProducts.length; i++) ...[
                    _RateRow(
                      name: MockData.loanProducts[i].name,
                      rate: '${MockData.loanProducts[i].interestRateFrom}%',
                      apr: '${MockData.loanProducts[i].aprFrom}%',
                      fee: MockData.loanProducts[i].processingFee,
                    ),
                    if (i != MockData.loanProducts.length - 1) const Divider(height: 1, indent: 16),
                  ],
                ],
              ),
            ),
            const SizedBox(height: 24),
            Text('Standard Charges', style: theme.textTheme.titleMedium),
            const SizedBox(height: 12),
            PremiumCard(
              child: Column(
                children: const [
                  _ChargeRow(label: 'GST', value: '18% on processing fee & applicable charges'),
                  Divider(height: 24),
                  _ChargeRow(label: 'Overdue Charges', value: '2% per month on overdue amount (not compounded)'),
                  Divider(height: 24),
                  _ChargeRow(label: 'Foreclosure Charges', value: 'Nil on floating-rate loans after 12 EMIs'),
                  Divider(height: 24),
                  _ChargeRow(label: 'Bounce Charges', value: '₹500 + GST per instance'),
                  Divider(height: 24),
                  _ChargeRow(label: 'Duplicate Statement', value: '₹200 + GST'),
                ],
              ),
            ),
            const SizedBox(height: 24),
            Text('Policies & Codes', style: theme.textTheme.titleMedium),
            const SizedBox(height: 12),
            const _PolicySection(
              title: 'Fair Practices Code',
              body:
                  'We are committed to transparent, fair, and ethical lending practices as prescribed by the RBI\'s Fair Practices Code for NBFCs. This includes clear communication of all terms before loan sanction, no unsolicited pre-approved credit limit increases without consent, and no harassment during recovery — all recovery follows the RBI-mandated Code of Conduct.',
            ),
            const _PolicySection(
              title: 'Key Facts Statement (KFS)',
              body:
                  'As per RBI\'s October 2024 circular, every retail and MSME loan is accompanied by a standardized Key Facts Statement summarizing the loan amount, interest rate, APR, all fees, and repayment schedule in simple language before you sign — shown to you during every loan application in this app.',
            ),
            const _PolicySection(
              title: 'Digital Lending Guidelines',
              body:
                  'In line with RBI\'s Digital Lending Guidelines (2022), we obtain explicit, itemized consent for your loan terms, credit bureau checks, and communication preferences separately — never as a single bundled checkbox — and disclose the full cost of credit upfront with no hidden charges.',
            ),
            const _PolicySection(
              title: 'Data Privacy',
              body:
                  'Your personal and financial data, including KYC documents, is collected solely for loan processing and regulatory compliance. It is encrypted at rest and in transit, never sold to third parties, and retained only as long as required by law.',
            ),
            const SizedBox(height: 24),
            Text('Grievance Redressal', style: theme.textTheme.titleMedium),
            const SizedBox(height: 12),
            PremiumCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Principal Nodal Officer', style: theme.textTheme.titleSmall),
                  const SizedBox(height: 4),
                  Text('Ms. Anjali Rao · nodalofficer@nbfcpremium.in · 1800-266-4545', style: theme.textTheme.bodySmall),
                  const SizedBox(height: 16),
                  Text('Grievance Redressal Officer', style: theme.textTheme.titleSmall),
                  const SizedBox(height: 4),
                  Text('Mr. Rohan Mehta · grievance@nbfcpremium.in · 1800-266-4546', style: theme.textTheme.bodySmall),
                  const SizedBox(height: 16),
                  Text(
                    'If your complaint is not resolved within 30 days, you may escalate it to the RBI Ombudsman via cms.rbi.org.in.',
                    style: theme.textTheme.bodySmall,
                  ),
                ],
              ),
            ),
            const SizedBox(height: 24),
            Text('Legal Documents', style: theme.textTheme.titleMedium),
            const SizedBox(height: 12),
            PremiumCard(
              padding: EdgeInsets.zero,
              child: Column(
                children: const [
                  _DocumentTile(label: 'Terms of Service'),
                  Divider(height: 1, indent: 16),
                  _DocumentTile(label: 'Privacy Policy'),
                  Divider(height: 1, indent: 16),
                  _DocumentTile(label: 'Customer Charter'),
                  Divider(height: 1, indent: 16),
                  _DocumentTile(label: 'Sample Loan Agreement'),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _RateRow extends StatelessWidget {
  const _RateRow({required this.name, required this.rate, required this.apr, required this.fee});

  final String name;
  final String rate;
  final String apr;
  final String fee;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(name, style: theme.textTheme.titleSmall),
          const SizedBox(height: 6),
          Wrap(
            spacing: 16,
            runSpacing: 4,
            children: [
              _MiniStat(label: 'Rate', value: rate),
              _MiniStat(label: 'APR', value: apr),
              _MiniStat(label: 'Fee', value: fee),
            ],
          ),
        ],
      ),
    );
  }
}

class _MiniStat extends StatelessWidget {
  const _MiniStat({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Text.rich(
      TextSpan(
        children: [
          TextSpan(text: '$label: ', style: theme.textTheme.labelSmall),
          TextSpan(text: value, style: theme.textTheme.labelSmall?.copyWith(color: AppColors.textPrimaryLight, fontWeight: FontWeight.w700)),
        ],
      ),
    );
  }
}

class _ChargeRow extends StatelessWidget {
  const _ChargeRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label, style: theme.textTheme.bodyMedium),
        const SizedBox(width: 12),
        Flexible(child: Text(value, style: theme.textTheme.titleSmall, textAlign: TextAlign.right)),
      ],
    );
  }
}

class _PolicySection extends StatelessWidget {
  const _PolicySection({required this.title, required this.body});

  final String title;
  final String body;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Theme(
        data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
        child: PremiumCard(
          padding: EdgeInsets.zero,
          child: ExpansionTile(
            title: Text(title, style: theme.textTheme.titleSmall),
            childrenPadding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
            children: [
              Align(alignment: Alignment.centerLeft, child: Text(body, style: theme.textTheme.bodyMedium)),
            ],
          ),
        ),
      ),
    );
  }
}

class _DocumentTile extends StatelessWidget {
  const _DocumentTile({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: const Icon(Icons.description_rounded, color: AppColors.primary),
      title: Text(label),
      trailing: const Icon(Icons.chevron_right_rounded),
      onTap: () => showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        builder: (context) => DraggableScrollableSheet(
          initialChildSize: 0.6,
          minChildSize: 0.4,
          maxChildSize: 0.9,
          expand: false,
          builder: (context, controller) => Container(
            decoration: const BoxDecoration(
              color: AppColors.surfaceLight,
              borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.xl)),
            ),
            padding: const EdgeInsets.all(20),
            child: ListView(
              controller: controller,
              children: [
                Text(label, style: Theme.of(context).textTheme.headlineSmall),
                const SizedBox(height: 12),
                Text(
                  'This is a summary document for demonstration purposes. In production, this would display the full, legally reviewed $label applicable to your loan agreement, in accordance with RBI\'s Fair Practices Code and applicable Indian law.',
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
