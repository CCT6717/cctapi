import DOMPurify from 'dompurify';

export const sanitizeHtml = (html) => DOMPurify.sanitize(html || '');
