// browsers show the QR code rather than scan it
export const scanQrLogin:
  ((onScan: (id: string) => void) => Promise<void>) | undefined = undefined;
