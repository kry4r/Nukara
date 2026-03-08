export function canSendEmailCode({ email = '', countdown = 0, isLoading = false } = {}) {
  return Boolean(String(email).trim()) && Number(countdown) <= 0 && !isLoading
}
