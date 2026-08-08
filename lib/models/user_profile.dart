class UserProfile {
  final String id;
  final String fullName;
  final String email;
  final String phone;
  final String city;
  final String state;
  final String avatarInitials;
  final bool kycVerified;
  final String customerSince;

  const UserProfile({
    required this.id,
    required this.fullName,
    required this.email,
    required this.phone,
    required this.city,
    required this.state,
    required this.avatarInitials,
    required this.kycVerified,
    required this.customerSince,
  });
}
