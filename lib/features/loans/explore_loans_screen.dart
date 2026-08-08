import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../mock/mock_data.dart';
import '../home/widgets/loan_product_card.dart';

class ExploreLoansScreen extends StatelessWidget {
  const ExploreLoansScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Explore Loans')),
      body: SafeArea(
        child: GridView.builder(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 120),
          itemCount: MockData.loanProducts.length,
          gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: 2,
            mainAxisSpacing: 14,
            crossAxisSpacing: 14,
            childAspectRatio: 1.35,
          ),
          itemBuilder: (context, i) {
            final product = MockData.loanProducts[i];
            return LoanProductCard(
              product: product,
              onTap: () => context.push('/loans/${product.id}'),
            );
          },
        ),
      ),
    );
  }
}
