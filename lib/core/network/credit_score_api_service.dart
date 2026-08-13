import 'api_client.dart';

class CreditScoreResult {
  final int score;
  final String band;
  final double preApprovedAmount;

  const CreditScoreResult({required this.score, required this.band, required this.preApprovedAmount});

  factory CreditScoreResult.fromJson(Map<String, dynamic> json) => CreditScoreResult(
        score: json['score'] as int? ?? 0,
        band: json['band'] as String? ?? '',
        preApprovedAmount: (json['preApprovedAmount'] as num?)?.toDouble() ?? 0,
      );
}

/// Wraps `GET /auth/credit-score` (`internal/loans/customer_handler.go`
/// `CustomerCreditScoreHandler`) — the customer's real, stable CIBIL
/// score (the same one used for actual loan eligibility/sanction) plus a
/// derived pre-approved amount, replacing MockData's hardcoded 785 /
/// "Excellent" / fake pre-approved offer.
class CreditScoreApiService {
  CreditScoreApiService._();

  static Future<CreditScoreResult> get(String token) async {
    final res = await ApiClient.get('/auth/credit-score', token: token);
    return CreditScoreResult.fromJson(res);
  }
}
