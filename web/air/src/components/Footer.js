import React, { useEffect, useRef, useState } from 'react';

import { Container, Segment } from 'semantic-ui-react';
import { getFooterHTML, getSystemName } from '../helpers';
import { EXTERNAL_URLS } from '../constants';
import DOMPurify from 'dompurify';

const Footer = () => {
  const systemName = getSystemName();
  const [footer, setFooter] = useState(getFooterHTML());
  const remainCheckTimesRef = useRef(5);

  const loadFooter = () => {
    let footer_html = localStorage.getItem('footer_html');
    if (footer_html) {
      setFooter(footer_html);
    }
  };

  useEffect(() => {
    const timer = setInterval(() => {
      if (remainCheckTimesRef.current <= 0) {
        clearInterval(timer);
        return;
      }
      remainCheckTimesRef.current--;
      loadFooter();
    }, 200);
    return () => clearInterval(timer);
  }, []);

  return (
    <Segment vertical>
      <Container textAlign='center'>
        {footer ? (
          <div
            className='custom-footer'
            dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(footer) }}
          ></div>
        ) : (
          <div className='custom-footer'>
            <a
              href={EXTERNAL_URLS.GITHUB_REPO}
              target='_blank'
              rel='noreferrer'
            >
              {systemName} {process.env.REACT_APP_VERSION}{' '}
            </a>
            由{' '}
            <a href={EXTERNAL_URLS.GITHUB_JUSTSONG} target='_blank' rel='noreferrer'>
              JustSong
            </a>{' '}
            构建，主题 air 来自{' '}
            <a href={EXTERNAL_URLS.GITHUB_CALON} target='_blank' rel='noreferrer'>
              Calon
            </a>{' '}，源代码遵循{' '}
            <a href={EXTERNAL_URLS.MIT_LICENSE}>
              MIT 协议
            </a>
          </div>
        )}
      </Container>
    </Segment>
  );
};

export default Footer;
