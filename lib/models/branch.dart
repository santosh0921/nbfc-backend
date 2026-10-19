/// A physical branch location (`GET /auth/branches`,
/// `internal/loans/branches.go`) — used by the Gold Loan apply flow, where
/// the collateral itself has to be physically brought in and appraised,
/// unlike every other loan category here which is handled entirely
/// remotely. A fixed list on the backend, not something customers or
/// admins manage.
class Branch {
  const Branch({required this.id, required this.name, required this.city});

  final String id;
  final String name;
  final String city;

  factory Branch.fromJson(Map<String, dynamic> json) => Branch(
        id: json['id'] as String? ?? '',
        name: json['name'] as String? ?? '',
        city: json['city'] as String? ?? '',
      );
}
