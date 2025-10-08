[Setup]
AppName=TaskTracker
AppVersion=1.0.0
AppPublisher=Khedes
DefaultDirName={autopf}\TaskTracker
DefaultGroupName=TaskTracker
OutputDir=installer_output
OutputBaseFilename=TaskTracker-Setup
; Kompression
Compression=lzma2
SolidCompression=yes

[Files]
; Exe ins Installationsverzeichnis
Source: "TaskTracker.exe"; DestDir: "{app}"; Flags: ignoreversion
; Icon mitliefern (optional, falls du es brauchst)
Source: "TaskTracker.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; Startmenü-Eintrag
Name: "{group}\TaskTracker"; Filename: "{app}\TaskTracker.exe"; IconFilename: "{app}\TaskTracker.ico"
; Optional: Desktop-Verknüpfung
Name: "{commondesktop}\TaskTracker"; Filename: "{app}\TaskTracker.exe"; IconFilename: "{app}\TaskTracker.ico"; Tasks: desktopicon

[Tasks]
; Checkbox während Installation für Desktop-Icon
Name: "desktopicon"; Description: "Create a &desktop icon"; GroupDescription: "Additional icons:"

[Run]
; Optional: App nach Installation starten
Filename: "{app}\TaskTracker.exe"; Description: "Launch TaskTracker"; Flags: nowait postinstall skipifsilent