import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../../mock/marketplace_listings.dart';
import '../../models/offer_banner.dart';

/// Real category listing screen for any [MarketplaceItem] — property
/// listings, vehicle listings, live auctions, insurance plans, or
/// investment products, each with a working detail sheet. Property
/// Worth gets its own instant-valuation calculator instead of a list,
/// since that's a tool rather than a catalog.
class MarketplaceCategoryScreen extends StatelessWidget {
  const MarketplaceCategoryScreen({super.key, required this.item});

  final MarketplaceItem item;

  @override
  Widget build(BuildContext context) {
    if (item.id == 'property_worth') {
      return _PropertyWorthScreen(item: item);
    }

    final listings = MarketplaceListings.byCategory[item.id] ?? const [];
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: Text(item.title)),
      body: SafeArea(
        child: listings.isEmpty
            ? Center(child: Text('No listings available right now.', style: theme.textTheme.bodyMedium))
            : ListView.separated(
                padding: const EdgeInsets.all(20),
                itemCount: listings.length,
                separatorBuilder: (_, __) => const SizedBox(height: 12),
                itemBuilder: (context, i) => _ListingCard(item: item, listing: listings[i]),
              ),
      ),
    );
  }
}

class _ListingCard extends StatelessWidget {
  const _ListingCard({required this.item, required this.listing});

  final MarketplaceItem item;
  final MarketplaceListing listing;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return PremiumCard(
      onTap: () => _showDetail(context),
      child: Row(
        children: [
          Container(
            width: 56,
            height: 56,
            decoration: BoxDecoration(
              gradient: LinearGradient(colors: item.gradient),
              borderRadius: BorderRadius.circular(AppRadius.md),
            ),
            child: Icon(listing.icon, color: Colors.white, size: 26),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(listing.title, style: theme.textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                Text(listing.subtitle, style: theme.textTheme.bodySmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                const SizedBox(height: 4),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                  decoration: BoxDecoration(
                    color: item.gradient.last.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(AppRadius.pill),
                  ),
                  child: Text(listing.tag, style: theme.textTheme.labelSmall?.copyWith(color: item.gradient.last, fontWeight: FontWeight.w700)),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Text(listing.price, style: theme.textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
        ],
      ),
    );
  }

  void _showDetail(BuildContext context) {
    showModalBottomSheet<void>(
      context: context,
      builder: (context) => Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 52,
                  height: 52,
                  decoration: BoxDecoration(gradient: LinearGradient(colors: item.gradient), borderRadius: BorderRadius.circular(AppRadius.md)),
                  child: Icon(listing.icon, color: Colors.white),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(listing.title, style: Theme.of(context).textTheme.titleMedium, maxLines: 1, overflow: TextOverflow.ellipsis),
                      Text(listing.price, style: Theme.of(context).textTheme.titleSmall?.copyWith(color: AppColors.primary), maxLines: 1, overflow: TextOverflow.ellipsis),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            Text(listing.subtitle, style: Theme.of(context).textTheme.bodyMedium),
            const SizedBox(height: 20),
            ElevatedButton(
              onPressed: () {
                Navigator.of(context).pop();
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Interest registered for ${listing.title}. Our team will contact you.')));
              },
              style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(50)),
              child: const Text('Express Interest'),
            ),
          ],
        ),
      ),
    );
  }
}

class _PropertyWorthScreen extends StatefulWidget {
  const _PropertyWorthScreen({required this.item});

  final MarketplaceItem item;

  @override
  State<_PropertyWorthScreen> createState() => _PropertyWorthScreenState();
}

class _PropertyWorthScreenState extends State<_PropertyWorthScreen> {
  final _formKey = GlobalKey<FormState>();
  final _areaController = TextEditingController();
  String _city = 'Pune';
  String _propertyType = 'Apartment';
  double? _estimatedValue;

  static const _cities = ['Pune', 'Mumbai', 'Bengaluru', 'Delhi', 'Hyderabad', 'Chennai'];
  static const _types = ['Apartment', 'Villa', 'Plot', 'Commercial'];

  static const Map<String, double> _rateSqft = {
    'Pune': 7200,
    'Mumbai': 21500,
    'Bengaluru': 8600,
    'Delhi': 12800,
    'Hyderabad': 6900,
    'Chennai': 7800,
  };

  @override
  void dispose() {
    _areaController.dispose();
    super.dispose();
  }

  void _estimate() {
    if (!(_formKey.currentState?.validate() ?? false)) return;
    final area = double.parse(_areaController.text);
    final rate = _rateSqft[_city] ?? 8000;
    final typeMultiplier = switch (_propertyType) {
      'Villa' => 1.25,
      'Commercial' => 1.4,
      'Plot' => 0.7,
      _ => 1.0,
    };
    setState(() => _estimatedValue = area * rate * typeMultiplier);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Property Worth')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            Text('Get an instant estimate of your property\'s market value.', style: theme.textTheme.bodyMedium),
            const SizedBox(height: 20),
            Form(
              key: _formKey,
              child: PremiumCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('City', style: theme.textTheme.titleSmall),
                    const SizedBox(height: 8),
                    DropdownButtonFormField<String>(
                      value: _city,
                      items: [for (final c in _cities) DropdownMenuItem(value: c, child: Text(c))],
                      onChanged: (v) => setState(() => _city = v ?? _city),
                      decoration: _dropdownDecoration(),
                    ),
                    const SizedBox(height: 16),
                    Text('Property Type', style: theme.textTheme.titleSmall),
                    const SizedBox(height: 8),
                    DropdownButtonFormField<String>(
                      value: _propertyType,
                      items: [for (final t in _types) DropdownMenuItem(value: t, child: Text(t))],
                      onChanged: (v) => setState(() => _propertyType = v ?? _propertyType),
                      decoration: _dropdownDecoration(),
                    ),
                    const SizedBox(height: 16),
                    Text('Area (sq.ft)', style: theme.textTheme.titleSmall),
                    const SizedBox(height: 8),
                    TextFormField(
                      controller: _areaController,
                      keyboardType: TextInputType.number,
                      inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                      validator: (v) => (v == null || v.isEmpty) ? 'Enter the built-up area' : null,
                      decoration: _dropdownDecoration(hint: 'e.g. 950'),
                    ),
                    const SizedBox(height: 16),
                    ElevatedButton(
                      onPressed: _estimate,
                      style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(50)),
                      child: const Text('Get Estimate'),
                    ),
                  ],
                ),
              ),
            ),
            if (_estimatedValue != null) ...[
              const SizedBox(height: 20),
              PremiumCard(
                child: Column(
                  children: [
                    Text('Estimated Market Value', style: theme.textTheme.bodyMedium),
                    const SizedBox(height: 6),
                    Text(
                      '₹${(_estimatedValue! / 100000).toStringAsFixed(1)} Lakh',
                      style: theme.textTheme.displaySmall?.copyWith(color: AppColors.primary, fontWeight: FontWeight.w800),
                    ),
                    const SizedBox(height: 4),
                    Text('Based on current $_city market rates for $_propertyType properties.', style: theme.textTheme.labelSmall, textAlign: TextAlign.center),
                  ],
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  InputDecoration _dropdownDecoration({String? hint}) => InputDecoration(
        hintText: hint,
        filled: true,
        fillColor: AppColors.backgroundLight,
        contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        border: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.borderLight)),
        enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.borderLight)),
        focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.primary, width: 1.6)),
      );
}
