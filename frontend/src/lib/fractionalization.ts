export interface FractionCalculation {
  totalSupply: number;
  fractionCount: number;
  unitsPerFraction: number;
  pricePerFraction: number;
  totalMarketValue: number;
  pricePerUnit: number;
}

export function calculateFractionDetails(
  totalSupply: number,
  fractionCount: number,
  pricePerFraction: number
): FractionCalculation {
  const unitsPerFraction = totalSupply / fractionCount;
  const totalMarketValue = fractionCount * pricePerFraction;
  const pricePerUnit = pricePerFraction / unitsPerFraction;

  return {
    totalSupply,
    fractionCount,
    unitsPerFraction,
    pricePerFraction,
    totalMarketValue,
    pricePerUnit,
  };
}

export function validateFractionConfiguration(
  totalSupply: number,
  fractionCount: number,
  pricePerFraction: number
): string | null {
  if (fractionCount <= 0) {
    return "Fraction count must be greater than 0";
  }

  if (fractionCount > totalSupply) {
    return "Fraction count cannot exceed total supply";
  }

  if (pricePerFraction <= 0) {
    return "Price per fraction must be greater than 0";
  }

  const unitsPerFraction = totalSupply / fractionCount;
  if (unitsPerFraction < 1) {
    return "Units per fraction cannot be less than 1";
  }

  return null;
}

export function recommendedFractionCount(totalSupply: number): number {
  if (totalSupply <= 100) return 10;
  if (totalSupply <= 1000) return 100;
  if (totalSupply <= 10000) return 1000;
  return 10000;
}

export function formatFractionPrice(stroops: number, decimals = 2): string {
  return (stroops / 1_0000000).toFixed(decimals);
}

export function parseStroopsInput(input: string): number {
  return Math.round(parseFloat(input) * 1_0000000);
}
