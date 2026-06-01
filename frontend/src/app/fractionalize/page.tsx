"use client";

import { useState } from "react";
import { Header } from "@/components/Header";
import { FractionalizationWizard } from "@/components/fractionalization/FractionalizationWizard";
import { StellarWallet } from "@/lib/stellar";

export default function FractionalizePage() {
  const [wallet, setWallet] = useState<StellarWallet | undefined>();

  const handleWalletConnected = (connectedWallet: StellarWallet) => {
    setWallet(connectedWallet);
  };

  const handleWalletDisconnected = () => {
    setWallet(undefined);
  };

  return (
    <div className="min-h-screen bg-background">
      <Header 
        wallet={wallet}
        onWalletConnected={handleWalletConnected}
        onWalletDisconnected={handleWalletDisconnected}
      />

      <main className="container mx-auto px-4 py-8">
        <div className="max-w-4xl mx-auto">
          <div className="mb-8">
            <h1 className="text-4xl font-bold mb-2">Asset Fractionalization</h1>
            <p className="text-lg text-muted-foreground">
              Split your assets into tradeable fractions to increase liquidity and accessibility
            </p>
          </div>

          {wallet ? (
            <FractionalizationWizard wallet={wallet} />
          ) : (
            <div className="bg-muted rounded-lg p-12 text-center">
              <p className="text-lg text-muted-foreground mb-4">
                Connect your wallet to begin fractionalization
              </p>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
