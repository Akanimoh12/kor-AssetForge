"use client";

import { FractionalizationConfig } from "./FractionalizationWizard";

interface PreviewStepProps {
  config: FractionalizationConfig;
  onPreviewComplete: () => void;
  onBack: () => void;
}

export function PreviewStep({
  config,
  onPreviewComplete,
  onBack,
}: PreviewStepProps) {
  const fractionValue = (config.totalSupply / config.fractionCount).toFixed(2);
  const totalValue = (config.fractionCount * config.pricePerFraction).toFixed(2);
  const averagePrice = (config.pricePerFraction * (config.totalSupply / config.fractionCount)).toFixed(2);

  return (
    <div className="space-y-6">
      <div className="bg-card rounded-lg border p-6">
        <h2 className="text-2xl font-semibold mb-2">Step 3: Preview</h2>
        <p className="text-muted-foreground mb-6">
          Review your fractionalization configuration before confirming
        </p>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div className="space-y-4">
            <div className="bg-muted/50 rounded p-4">
              <h3 className="font-semibold mb-4">Asset Details</h3>
              <div className="space-y-3 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Name:</span>
                  <span className="font-medium">{config.assetName}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Symbol:</span>
                  <span className="font-medium">{config.assetSymbol}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Total Supply:</span>
                  <span className="font-medium">{config.totalSupply.toLocaleString()}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Asset ID:</span>
                  <span className="font-medium">#{config.assetId}</span>
                </div>
              </div>
            </div>
          </div>

          <div className="space-y-4">
            <div className="bg-primary/10 border border-primary/20 rounded p-4">
              <h3 className="font-semibold mb-4">Fraction Configuration</h3>
              <div className="space-y-3 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Total Fractions:</span>
                  <span className="font-medium">{config.fractionCount.toLocaleString()}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Units per Fraction:</span>
                  <span className="font-medium">{fractionValue}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Price per Fraction:</span>
                  <span className="font-medium">{config.pricePerFraction} stroops</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Total Market Value:</span>
                  <span className="font-medium">{totalValue} stroops</span>
                </div>
                <div className="pt-3 border-t border-primary/20">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Average Price/Unit:</span>
                    <span className="font-medium">{averagePrice} stroops</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="mt-8 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-900/50 rounded p-4">
          <h4 className="font-semibold text-blue-900 dark:text-blue-100 mb-2">Preview Summary</h4>
          <p className="text-sm text-blue-800 dark:text-blue-200">
            Your asset "{config.assetName}" will be divided into {config.fractionCount.toLocaleString()} tradeable fractions. 
            Each fraction represents {fractionValue} units and will be priced at {config.pricePerFraction} stroops. 
            The total market value will be {totalValue} stroops.
          </p>
        </div>

        <div className="mt-8 flex gap-4">
          <button
            onClick={onBack}
            className="flex-1 border border-input bg-background text-foreground py-2 rounded-lg font-medium hover:bg-muted transition"
          >
            Back to Configuration
          </button>
          <button
            onClick={onPreviewComplete}
            className="flex-1 bg-primary text-primary-foreground py-2 rounded-lg font-medium hover:bg-primary/90 transition"
          >
            Proceed to Confirmation
          </button>
        </div>
      </div>
    </div>
  );
}
