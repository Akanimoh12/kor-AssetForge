"use client";

import { useState } from "react";
import { FractionalizationConfig } from "./FractionalizationWizard";

interface FractionConfigStepProps {
  config: FractionalizationConfig;
  onConfigurationComplete: (config: FractionalizationConfig) => void;
}

export function FractionConfigStep({
  config,
  onConfigurationComplete,
}: FractionConfigStepProps) {
  const [fractionCount, setFractionCount] = useState(config.fractionCount);
  const [pricePerFraction, setPricePerFraction] = useState(config.pricePerFraction);

  const fractionValue = (config.totalSupply / fractionCount).toFixed(2);
  const totalValue = (fractionCount * pricePerFraction).toFixed(2);

  const handleContinue = () => {
    onConfigurationComplete({
      ...config,
      fractionCount,
      pricePerFraction,
    });
  };

  return (
    <div className="space-y-6">
      <div className="bg-card rounded-lg border p-6">
        <h2 className="text-2xl font-semibold mb-2">Step 2: Configure Fractions</h2>
        <p className="text-muted-foreground mb-6">
          Set up the parameters for fractionalization
        </p>

        <div className="space-y-6">
          <div className="bg-muted/50 rounded p-4">
            <h3 className="font-semibold mb-3">Asset Information</h3>
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <p className="text-muted-foreground">Asset Name</p>
                <p className="font-medium">{config.assetName}</p>
              </div>
              <div>
                <p className="text-muted-foreground">Symbol</p>
                <p className="font-medium">{config.assetSymbol}</p>
              </div>
              <div>
                <p className="text-muted-foreground">Total Supply</p>
                <p className="font-medium">{config.totalSupply.toLocaleString()}</p>
              </div>
              <div>
                <p className="text-muted-foreground">Asset Type</p>
                <p className="font-medium">Tokenized Asset</p>
              </div>
            </div>
          </div>

          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-2">
                Number of Fractions
              </label>
              <input
                type="number"
                min="100"
                max="1000000"
                step="100"
                value={fractionCount}
                onChange={(e) => setFractionCount(Number(e.target.value))}
                className="w-full px-3 py-2 border rounded-lg bg-background"
              />
              <p className="text-xs text-muted-foreground mt-1">
                Each fraction will represent {fractionValue} units of the asset
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium mb-2">
                Price Per Fraction (in stroops)
              </label>
              <input
                type="number"
                min="1"
                step="0.01"
                value={pricePerFraction}
                onChange={(e) => setPricePerFraction(Number(e.target.value))}
                className="w-full px-3 py-2 border rounded-lg bg-background"
              />
              <p className="text-xs text-muted-foreground mt-1">
                Set the trading price for each fraction
              </p>
            </div>
          </div>

          <div className="bg-primary/10 border border-primary/20 rounded p-4">
            <h4 className="font-semibold mb-3">Calculation Summary</h4>
            <div className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Total Fractions:</span>
                <span className="font-medium">{fractionCount.toLocaleString()}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Units per Fraction:</span>
                <span className="font-medium">{fractionValue}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Price per Fraction:</span>
                <span className="font-medium">{pricePerFraction} stroops</span>
              </div>
              <div className="border-t border-primary/20 pt-2 mt-2 flex justify-between font-semibold">
                <span>Total Market Value:</span>
                <span>{totalValue} stroops</span>
              </div>
            </div>
          </div>
        </div>

        <div className="mt-8 flex gap-4">
          <button
            onClick={handleContinue}
            className="flex-1 bg-primary text-primary-foreground py-2 rounded-lg font-medium hover:bg-primary/90 transition"
          >
            Continue to Preview
          </button>
        </div>
      </div>
    </div>
  );
}
