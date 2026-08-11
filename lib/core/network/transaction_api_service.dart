import 'api_client.dart';
import '../../models/transaction.dart';

/// Wraps `GET /auth/transactions`
/// (`internal/loans/transactions_handler.go`) — the customer's real,
/// unified payment/disbursal ledger across every loan they've ever had.
class TransactionApiService {
  TransactionApiService._();

  static Future<List<AppTransaction>> mine(String token) async {
    final res = await ApiClient.getList('/auth/transactions', token: token);
    return res.map((e) => AppTransaction.fromJson(e as Map<String, dynamic>)).toList();
  }
}
