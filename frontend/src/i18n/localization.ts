import { useTranslation } from 'react-i18next';

export const useLocalization = () => {
  const { i18n } = useTranslation();

  const formatDate = (date: Date | string): string => {
    const d = typeof date === 'string' ? new Date(date) : date;
    return d.toLocaleDateString(i18n.language, {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    });
  };

  const formatTime = (date: Date | string): string => {
    const d = typeof date === 'string' ? new Date(date) : date;
    return d.toLocaleTimeString(i18n.language, {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  };

  const formatDateTime = (date: Date | string): string => {
    return `${formatDate(date)} ${formatTime(date)}`;
  };

  const formatNumber = (value: number, minimumFractionDigits = 0, maximumFractionDigits = 2): string => {
    return new Intl.NumberFormat(i18n.language, {
      minimumFractionDigits,
      maximumFractionDigits,
    }).format(value);
  };

  const formatCurrency = (value: number, currency = 'USD'): string => {
    return new Intl.NumberFormat(i18n.language, {
      style: 'currency',
      currency,
    }).format(value);
  };

  const formatPercent = (value: number, minimumFractionDigits = 0): string => {
    return new Intl.NumberFormat(i18n.language, {
      style: 'percent',
      minimumFractionDigits,
    }).format(value);
  };

  return {
    formatDate,
    formatTime,
    formatDateTime,
    formatNumber,
    formatCurrency,
    formatPercent,
  };
};
