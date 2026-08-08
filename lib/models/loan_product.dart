import 'package:flutter/material.dart';

enum LoanCategory {
  housing,
  personal,
  business,
  gold,
  education,
  msme,
  rural,
  farmer,
  tractor,
  vehicle,
  commercialVehicle,
  warehouseFinance,
  loanAgainstProperty,
  twoWheeler,
  instantLoan,
}

class LoanProduct {
  final String id;
  final LoanCategory category;
  final String name;
  final String shortDescription;
  final IconData icon;
  final List<Color> gradient;
  final double interestRateFrom;
  final double aprFrom;
  final String tenureRange;
  final String maxAmount;
  final String processingFee;
  final List<String> benefits;
  final List<String> eligibility;
  final List<String> documents;
  final List<Map<String, String>> faqs;

  const LoanProduct({
    required this.id,
    required this.category,
    required this.name,
    required this.shortDescription,
    required this.icon,
    required this.gradient,
    required this.interestRateFrom,
    required this.aprFrom,
    required this.tenureRange,
    required this.maxAmount,
    required this.processingFee,
    required this.benefits,
    required this.eligibility,
    required this.documents,
    required this.faqs,
  });
}
