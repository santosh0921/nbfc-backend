import 'package:flutter/material.dart';

class OfferBanner {
  final String id;
  final String title;
  final String subtitle;
  final String ctaLabel;
  final List<Color> gradient;
  final IconData icon;
  final String? badge;

  const OfferBanner({
    required this.id,
    required this.title,
    required this.subtitle,
    required this.ctaLabel,
    required this.gradient,
    required this.icon,
    this.badge,
  });
}

class AppNotification {
  final String id;
  final String title;
  final String message;
  final DateTime time;
  final bool read;
  final IconData icon;

  const AppNotification({
    required this.id,
    required this.title,
    required this.message,
    required this.time,
    required this.read,
    required this.icon,
  });
}

class FinancialTip {
  final String id;
  final String title;
  final String summary;
  final String readTime;
  final List<Color> gradient;

  const FinancialTip({
    required this.id,
    required this.title,
    required this.summary,
    required this.readTime,
    required this.gradient,
  });
}
