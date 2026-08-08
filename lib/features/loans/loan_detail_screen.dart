import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../../mock/mock_data.dart';
import '../../models/loan_product.dart';
import 'widgets/emi_calculator.dart';

class LoanDetailScreen extends StatelessWidget {
  final String loanId;

  const LoanDetailScreen({super.key, required this.loanId});

  @override
  Widget build(BuildContext context) {
    final LoanProduct product = MockData.loanProducts.firstWhere((p) => p.id == loanId);
    final theme = Theme.of(context);
    final related = MockData.loanProducts.where((p) => p.id != loanId).take(4).toList();

    return Scaffold(
      body: CustomScrollView(
        slivers: [
          SliverAppBar(
            expandedHeight: 220,
            pinned: true,
            backgroundColor: product.gradient.last,
            iconTheme: const IconThemeData(color: Colors.white),
            flexibleSpace: FlexibleSpaceBar(
              background: Container(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    colors: product.gradient,
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                  ),
                ),
                child: Stack(
                  children: [
                    Positioned(
                      right: -20,
                      bottom: -10,
                      child: Icon(product.icon, size: 180, color: Colors.white.withValues(alpha: 0.14)),
                    ),
                    Padding(
                      padding: const EdgeInsets.fromLTRB(20, 80, 20, 20),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisAlignment: MainAxisAlignment.end,
                        children: [
                          Text(
                            product.name,
                            style: theme.textTheme.headlineLarge?.copyWith(color: Colors.white),
                          ),
                          const SizedBox(height: 6),
                          Text(
                            product.shortDescription,
                            style: theme.textTheme.bodyMedium?.copyWith(color: Colors.white.withValues(alpha: 0.9)),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
          SliverPadding(
            padding: const EdgeInsets.fromLTRB(20, 20, 20, 140),
            sliver: SliverList(
              delegate: SliverChildListDelegate([
                Row(
                  children: [
                    _StatChip(label: 'Interest', value: '${product.interestRateFrom}%*'),
                    const SizedBox(width: 10),
                    _StatChip(label: 'APR', value: '${product.aprFrom}%*'),
                    const SizedBox(width: 10),
                    _StatChip(label: 'Max Amount', value: product.maxAmount),
                  ],
                ),
                const SizedBox(height: 20),
                PremiumCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      _DetailRow(label: 'Tenure', value: product.tenureRange),
                      const Divider(height: 20),
                      _DetailRow(label: 'Processing Fee', value: product.processingFee),
                      const Divider(height: 20),
                      _DetailRow(label: 'Interest Rate', value: 'from ${product.interestRateFrom}% p.a.'),
                      const Divider(height: 20),
                      _DetailRow(label: 'APR', value: 'from ${product.aprFrom}% p.a.'),
                    ],
                  ),
                ),
                const SizedBox(height: 24),
                Text('Key Benefits', style: theme.textTheme.titleMedium),
                const SizedBox(height: 12),
                ...product.benefits.map((b) => _BulletRow(text: b, icon: Icons.check_circle_rounded, color: AppColors.success)),
                const SizedBox(height: 24),
                Text('Eligibility', style: theme.textTheme.titleMedium),
                const SizedBox(height: 12),
                ...product.eligibility.map((b) => _BulletRow(text: b, icon: Icons.task_alt_rounded, color: AppColors.primary)),
                const SizedBox(height: 24),
                Text('Documents Required', style: theme.textTheme.titleMedium),
                const SizedBox(height: 12),
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: product.documents
                      .map((d) => Chip(label: Text(d), avatar: const Icon(Icons.description_rounded, size: 16)))
                      .toList(),
                ),
                const SizedBox(height: 24),
                EmiCalculator(interestRate: product.interestRateFrom),
                const SizedBox(height: 24),
                Text('Frequently Asked Questions', style: theme.textTheme.titleMedium),
                const SizedBox(height: 12),
                ...product.faqs.map((f) => _FaqTile(question: f['q']!, answer: f['a']!)),
                const SizedBox(height: 24),
                Text('Related Products', style: theme.textTheme.titleMedium),
                const SizedBox(height: 12),
                SizedBox(
                  height: 118,
                  child: ListView.separated(
                    scrollDirection: Axis.horizontal,
                    itemCount: related.length,
                    separatorBuilder: (_, __) => const SizedBox(width: 12),
                    itemBuilder: (context, i) {
                      final r = related[i];
                      return GestureDetector(
                        onTap: () => context.push('/loans/${r.id}'),
                        child: Container(
                          width: 130,
                          padding: const EdgeInsets.all(14),
                          decoration: BoxDecoration(
                            borderRadius: BorderRadius.circular(AppRadius.md),
                            gradient: LinearGradient(colors: r.gradient),
                          ),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Icon(r.icon, color: Colors.white),
                              const Spacer(),
                              Text(
                                r.name,
                                style: theme.textTheme.labelLarge?.copyWith(color: Colors.white),
                                maxLines: 2,
                              ),
                            ],
                          ),
                        ),
                      );
                    },
                  ),
                ),
              ]),
            ),
          ),
        ],
      ),
      bottomNavigationBar: SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 12),
          child: ElevatedButton(
            onPressed: () => context.push('/quick-apply', extra: product),
            style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(52)),
            child: Text('Apply Now for ${product.name}'),
          ),
        ),
      ),
    );
  }
}

class _StatChip extends StatelessWidget {
  final String label;
  final String value;

  const _StatChip({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Expanded(
      child: PremiumCard(
        padding: const EdgeInsets.symmetric(vertical: 14, horizontal: 10),
        child: Column(
          children: [
            Text(value, style: theme.textTheme.titleMedium, textAlign: TextAlign.center, maxLines: 1),
            const SizedBox(height: 2),
            Text(label, style: theme.textTheme.labelSmall, textAlign: TextAlign.center),
          ],
        ),
      ),
    );
  }
}

class _DetailRow extends StatelessWidget {
  final String label;
  final String value;

  const _DetailRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label, style: theme.textTheme.bodyMedium),
        Text(value, style: theme.textTheme.titleSmall),
      ],
    );
  }
}

class _BulletRow extends StatelessWidget {
  final String text;
  final IconData icon;
  final Color color;

  const _BulletRow({required this.text, required this.icon, required this.color});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 18, color: color),
          const SizedBox(width: 10),
          Expanded(child: Text(text, style: Theme.of(context).textTheme.bodyMedium)),
        ],
      ),
    );
  }
}

class _FaqTile extends StatelessWidget {
  final String question;
  final String answer;

  const _FaqTile({required this.question, required this.answer});

  @override
  Widget build(BuildContext context) {
    return Theme(
      data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
      child: PremiumCard(
        padding: EdgeInsets.zero,
        child: ExpansionTile(
          title: Text(question, style: Theme.of(context).textTheme.titleSmall),
          childrenPadding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
          children: [
            Align(
              alignment: Alignment.centerLeft,
              child: Text(answer, style: Theme.of(context).textTheme.bodyMedium),
            ),
          ],
        ),
      ),
    );
  }
}
