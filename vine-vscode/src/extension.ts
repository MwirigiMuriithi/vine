import * as vscode from 'vscode';

export function activate(context: vscode.ExtensionContext): void {
  vscode.window.showInformationMessage('Vine Language extension activated');
}

export function deactivate(): void {
  // Reserved for cleanup later
}