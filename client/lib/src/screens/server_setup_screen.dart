import 'package:flutter/material.dart';
import 'package:meowgram_client/src/config/server_config_controller.dart';
import 'package:meowgram_client/src/widgets/meowgram_logo.dart';

/// First launch of a published mobile or desktop build: which meowGram server
/// to use. The router leaves this screen once the server has answered.
class ServerSetupScreen extends StatefulWidget {
  final ServerConfigController serverConfig;

  const ServerSetupScreen({super.key, required this.serverConfig});

  @override
  State<ServerSetupScreen> createState() => _ServerSetupScreenState();
}

class _ServerSetupScreenState extends State<ServerSetupScreen> {
  final _address = TextEditingController();

  @override
  void dispose() {
    _address.dispose();
    super.dispose();
  }

  Future<void> _connect() async {
    if (widget.serverConfig.isLoading) return;
    await widget.serverConfig.connect(_address.text);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding:
                const EdgeInsets.symmetric(horizontal: 24.0, vertical: 32.0),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 480),
              child: AnimatedBuilder(
                animation: widget.serverConfig,
                builder: (context, _) {
                  final loading = widget.serverConfig.isLoading;
                  final error = widget.serverConfig.error;
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      const Center(
                        child: MeowGramLogo(size: 96, elevated: true),
                      ),
                      const SizedBox(height: 20),
                      Text(
                        'Connect to your lounge',
                        textAlign: TextAlign.center,
                        style: theme.textTheme.headlineSmall?.copyWith(
                          fontWeight: FontWeight.w800,
                        ),
                      ),
                      const SizedBox(height: 6),
                      Text(
                        'Enter the address of your household\'s meowGram server.',
                        textAlign: TextAlign.center,
                        style: theme.textTheme.bodyMedium?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                      const SizedBox(height: 28),
                      TextField(
                        key: const Key('server-address'),
                        controller: _address,
                        enabled: !loading,
                        autofocus: true,
                        autocorrect: false,
                        keyboardType: TextInputType.url,
                        textInputAction: TextInputAction.go,
                        onSubmitted: (_) => _connect(),
                        decoration: InputDecoration(
                          labelText: 'Server address',
                          hintText: 'meow.example.home.arpa',
                          prefixIcon: const Icon(Icons.dns_rounded),
                          errorText: error,
                          errorMaxLines: 3,
                          border: OutlineInputBorder(
                            borderRadius: BorderRadius.circular(14),
                          ),
                        ),
                      ),
                      const SizedBox(height: 20),
                      FilledButton.icon(
                        onPressed: loading ? null : _connect,
                        icon: loading
                            ? const SizedBox(
                                width: 18,
                                height: 18,
                                child:
                                    CircularProgressIndicator(strokeWidth: 2),
                              )
                            : const Icon(Icons.arrow_forward_rounded, size: 20),
                        label: Text(
                          loading ? 'Connecting...' : 'Continue',
                          style: const TextStyle(
                              fontSize: 16, fontWeight: FontWeight.bold),
                        ),
                        style: FilledButton.styleFrom(
                          padding: const EdgeInsets.symmetric(vertical: 18),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(14),
                          ),
                        ),
                      ),
                      const SizedBox(height: 24),
                      Text(
                        'The server tells the app where to sign in. You can change it later from the sign-in screen.',
                        textAlign: TextAlign.center,
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.outline,
                        ),
                      ),
                    ],
                  );
                },
              ),
            ),
          ),
        ),
      ),
    );
  }
}
