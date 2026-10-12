; Windows installer for meowGram (Inno Setup 6). Built by the release workflow:
;   iscc /DAppVersion=1.1.0 /DSourceDir=<flutter Release folder> /DOutputDir=<dir> meowgram.iss
; Per-user install by default (no admin prompt); the user can choose all users.

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif
#ifndef SourceDir
  #define SourceDir "..\..\build\windows\x64\runner\Release"
#endif
#ifndef OutputDir
  #define OutputDir "..\..\build\installer"
#endif

[Setup]
; Never change AppId: it is how upgrades find the installed copy.
AppId={{D380A047-8B92-4D3D-8421-9543E89A6AF4}
AppName=meowGram
AppVersion={#AppVersion}
AppVerName=meowGram {#AppVersion}
AppPublisher=purrBrews
DefaultDirName={autopf}\meowGram
DefaultGroupName=meowGram
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#OutputDir}
OutputBaseFilename=meowGram-{#AppVersion}-setup
SetupIconFile=..\runner\resources\app_icon.ico
UninstallDisplayIcon={app}\meowgram_client.exe
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\meowGram"; Filename: "{app}\meowgram_client.exe"
Name: "{autodesktop}\meowGram"; Filename: "{app}\meowgram_client.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\meowgram_client.exe"; Description: "{cm:LaunchProgram,meowGram}"; Flags: nowait postinstall skipifsilent
