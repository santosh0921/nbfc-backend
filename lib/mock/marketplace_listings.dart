import 'package:flutter/material.dart';

class MarketplaceListing {
  final String title;
  final String subtitle;
  final String price;
  final String tag;
  final IconData icon;

  const MarketplaceListing({
    required this.title,
    required this.subtitle,
    required this.price,
    required this.tag,
    required this.icon,
  });
}

/// Mock listings shown per marketplace category id (see
/// [MockData.marketplaceItems]).
class MarketplaceListings {
  MarketplaceListings._();

  static const Map<String, List<MarketplaceListing>> byCategory = {
    'property_search': [
      MarketplaceListing(title: '2 BHK Apartment', subtitle: 'Baner, Pune · 950 sq.ft', price: '₹78 Lakh', tag: 'Ready to Move', icon: Icons.apartment_rounded),
      MarketplaceListing(title: '3 BHK Villa', subtitle: 'Whitefield, Bengaluru · 1800 sq.ft', price: '₹1.6 Cr', tag: 'New Launch', icon: Icons.villa_rounded),
      MarketplaceListing(title: '1 BHK Flat', subtitle: 'Andheri West, Mumbai · 550 sq.ft', price: '₹95 Lakh', tag: 'Resale', icon: Icons.home_work_rounded),
      MarketplaceListing(title: 'Plot – 2400 sq.ft', subtitle: 'Sarjapur Road, Bengaluru', price: '₹42 Lakh', tag: 'Corner Plot', icon: Icons.terrain_rounded),
    ],
    'two_wheeler_bazaar': [
      MarketplaceListing(title: 'Royal Enfield Classic 350', subtitle: '2022 · 8,400 km · Petrol', price: '₹1.45 Lakh', tag: 'Certified', icon: Icons.two_wheeler_rounded),
      MarketplaceListing(title: 'Honda Activa 6G', subtitle: '2023 · 3,200 km · Petrol', price: '₹68,000', tag: 'Single Owner', icon: Icons.two_wheeler_rounded),
      MarketplaceListing(title: 'TVS iQube Electric', subtitle: '2023 · 1,800 km · Electric', price: '₹92,000', tag: 'Like New', icon: Icons.electric_moped_rounded),
      MarketplaceListing(title: 'Bajaj Pulsar NS200', subtitle: '2021 · 12,600 km · Petrol', price: '₹1.05 Lakh', tag: 'Well Maintained', icon: Icons.two_wheeler_rounded),
    ],
    'auction_portal': [
      MarketplaceListing(title: 'Bank-Seized Apartment', subtitle: '2 BHK, Nashik · Auction closes in 4 days', price: 'Reserve ₹32 Lakh', tag: 'Live Auction', icon: Icons.gavel_rounded),
      MarketplaceListing(title: 'Commercial Shop', subtitle: 'Nagpur Main Road · 320 sq.ft', price: 'Reserve ₹18 Lakh', tag: 'Live Auction', icon: Icons.storefront_rounded),
      MarketplaceListing(title: 'Repossessed SUV', subtitle: 'Mahindra XUV500, 2019', price: 'Reserve ₹6.2 Lakh', tag: 'Ending Soon', icon: Icons.directions_car_rounded),
      MarketplaceListing(title: 'Agricultural Land', subtitle: '2 acres, Nashik district', price: 'Reserve ₹24 Lakh', tag: 'Upcoming', icon: Icons.landscape_rounded),
    ],
    'insurance': [
      MarketplaceListing(title: 'Health Insurance', subtitle: 'Cover up to ₹10 Lakh · Family floater', price: '₹699/month', tag: 'Cashless', icon: Icons.health_and_safety_rounded),
      MarketplaceListing(title: 'Term Life Insurance', subtitle: 'Cover up to ₹1 Cr · 30-year term', price: '₹499/month', tag: 'Tax Saver', icon: Icons.shield_rounded),
      MarketplaceListing(title: 'Motor Insurance', subtitle: 'Comprehensive · Zero depreciation', price: '₹350/month', tag: 'Instant Policy', icon: Icons.car_crash_rounded),
      MarketplaceListing(title: 'Loan Protection Plan', subtitle: 'Covers your outstanding loan on death/disability', price: '₹199/month', tag: 'Recommended', icon: Icons.verified_user_rounded),
    ],
    'investments': [
      MarketplaceListing(title: 'Fixed Deposit', subtitle: '1-year tenure · Senior citizen +0.5%', price: '7.5% p.a.', tag: 'Guaranteed', icon: Icons.savings_rounded),
      MarketplaceListing(title: 'Equity Mutual Fund', subtitle: 'Large-cap · 5-year avg. return', price: '12.4% p.a.', tag: 'High Growth', icon: Icons.trending_up_rounded),
      MarketplaceListing(title: 'Sovereign Gold Bond', subtitle: '8-year tenure · Govt. backed', price: '2.5% + gold price', tag: 'Tax Free', icon: Icons.diamond_rounded),
      MarketplaceListing(title: 'Recurring Deposit', subtitle: 'Monthly instalments · 3-year tenure', price: '7.1% p.a.', tag: 'Low Risk', icon: Icons.account_balance_rounded),
    ],
  };
}
