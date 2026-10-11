import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:meowgram_client/src/widgets/meowgram_logo.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('logo asset is bundled', () async {
    final data = await rootBundle.load(MeowGramLogo.assetPath);
    // PNG signature: \x89 P N G
    expect(data.getUint32(0), 0x89504E47);
  });

  testWidgets('MeowGramLogo renders the asset at the requested size',
      (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Center(child: MeowGramLogo(size: 64, elevated: true)),
      ),
    );

    final image = tester.widget<Image>(find.byType(Image));
    expect((image.image as AssetImage).assetName, MeowGramLogo.assetPath);
    expect(tester.getSize(find.byType(Image)), const Size(64, 64));
    expect(find.bySemanticsLabel('meowGram'), findsOneWidget);
  });
}
