import React from 'react';
import { useTranslation } from 'react-i18next';

const Chat = () => {
  const { t } = useTranslation();
  const chatLink = localStorage.getItem('chat_link');

  if (!chatLink) {
    return (
      <div style={{ padding: '2rem', textAlign: 'center', color: '#666' }}>
        {t('chat.no_link_configured')}
      </div>
    );
  }

  return (
    <iframe
      src={chatLink}
      style={{ width: '100%', height: '85vh', border: 'none' }}
      title="chat"
    />
  );
};

export default Chat;
