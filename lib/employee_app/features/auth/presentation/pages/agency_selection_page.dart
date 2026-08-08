import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/providers/agency_provider.dart';
import '../../../../core/router/route_names.dart';

class _Agency {
  const _Agency({required this.name, required this.city, required this.code});
  final String name;
  final String city;
  final String code;
}

const _agencies = [
  _Agency(name: 'ONEFIN — Baner Branch', city: 'Pune, Maharashtra', code: 'PUN01'),
  _Agency(name: 'ONEFIN — Andheri Branch', city: 'Mumbai, Maharashtra', code: 'MUM04'),
  _Agency(name: 'ONEFIN — Connaught Place', city: 'New Delhi', code: 'DEL02'),
  _Agency(name: 'ONEFIN — MG Road Branch', city: 'Bengaluru, Karnataka', code: 'BLR07'),
  _Agency(name: 'ONEFIN — Anna Nagar Branch', city: 'Chennai, Tamil Nadu', code: 'CHN03'),
  _Agency(name: 'ONEFIN — Salt Lake Branch', city: 'Kolkata, West Bengal', code: 'KOL05'),
];

/// First screen a device ever shows — the field officer picks the branch
/// / franchise they're operating under. Persisted locally so it's only
/// asked once per install.
class AgencySelectionPage extends StatefulWidget {
  const AgencySelectionPage({super.key});

  @override
  State<AgencySelectionPage> createState() => _AgencySelectionPageState();
}

class _AgencySelectionPageState extends State<AgencySelectionPage> {
  String _query = '';

  @override
  Widget build(BuildContext context) {
    final filtered = _agencies
        .where((a) => _query.isEmpty || a.name.toLowerCase().contains(_query.toLowerCase()) || a.city.toLowerCase().contains(_query.toLowerCase()))
        .toList();

    return Scaffold(
      backgroundColor: Colors.white,
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(AppSpacing.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: AppSpacing.md),
              Container(
                width: 64,
                height: 64,
                alignment: Alignment.center,
                decoration: BoxDecoration(color: AppColors.secondary, borderRadius: BorderRadius.circular(AppSpacing.radiusLg)),
                child: const Icon(Icons.apartment_rounded, color: AppColors.primary, size: 30),
              ),
              const SizedBox(height: AppSpacing.md),
              Text('Select Your Agency', style: Theme.of(context).textTheme.headlineSmall),
              const SizedBox(height: 4),
              Text(
                'Choose the branch you are registered under to continue.',
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppColors.textSecondary),
              ),
              const SizedBox(height: AppSpacing.lg),
              Container(
                height: 46,
                padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md),
                decoration: BoxDecoration(
                  color: AppColors.surfaceMuted,
                  borderRadius: BorderRadius.circular(AppSpacing.radiusMd),
                ),
                child: Row(
                  children: [
                    const Icon(Icons.search, size: 20, color: AppColors.textSecondary),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: TextField(
                        onChanged: (v) => setState(() => _query = v),
                        decoration: const InputDecoration(
                          hintText: 'Search branch or city',
                          border: InputBorder.none,
                          isDense: true,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: AppSpacing.md),
              Expanded(
                child: ListView.separated(
                  itemCount: filtered.length,
                  separatorBuilder: (_, __) => const SizedBox(height: AppSpacing.sm),
                  itemBuilder: (context, index) {
                    final agency = filtered[index];
                    return AppCard(
                      onTap: () async {
                        await context.read<AgencyProvider>().selectAgency(agency.name);
                        if (context.mounted) context.goNamed(RouteNames.login);
                      },
                      child: Row(
                        children: [
                          Container(
                            width: 42,
                            height: 42,
                            alignment: Alignment.center,
                            decoration: BoxDecoration(color: AppColors.primary.withValues(alpha: 0.14), borderRadius: BorderRadius.circular(AppSpacing.radiusSm)),
                            child: const Icon(Icons.store_mall_directory_outlined, color: AppColors.secondary),
                          ),
                          const SizedBox(width: AppSpacing.sm),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  agency.name,
                                  style: Theme.of(context).textTheme.titleSmall,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                                Text(
                                  '${agency.city} · ${agency.code}',
                                  style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ],
                            ),
                          ),
                          const Icon(Icons.chevron_right),
                        ],
                      ),
                    );
                  },
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
