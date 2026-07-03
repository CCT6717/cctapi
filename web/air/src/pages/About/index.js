import React, { useEffect, useState } from 'react';
import { Header, Segment } from 'semantic-ui-react';
import { API, showError } from '../../helpers';
import { EXTERNAL_URLS } from '../../constants';
import { marked } from 'marked';
import { sanitizeHtml } from '../../helpers/sanitize';

const About = () => {
  const [about, setAbout] = useState('');
  const [aboutLoaded, setAboutLoaded] = useState(false);

  const displayAbout = async (isMounted) => {
    setAbout(localStorage.getItem('about') || '');
    const res = await API.get('/api/about');
    if (!isMounted) return;
    const { success, message, data } = res.data;
    if (success) {
      let aboutContent = data;
      if (!data.startsWith('https://')) {
        aboutContent = sanitizeHtml(marked.parse(data));
      }
      setAbout(aboutContent);
      localStorage.setItem('about', aboutContent);
    } else {
      showError(message);
      setAbout('加载关于内容失败...');
    }
    setAboutLoaded(true);
  };

  useEffect(() => {
    let isMounted = true;
    const run = async (isMounted) => {
      await displayAbout(isMounted);
    };
    run(isMounted);
    return () => {
      isMounted = false;
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <>
      {
        aboutLoaded && about === '' ? <>
          <Segment>
            <Header as='h3'>关于</Header>
            <p>可在设置页面设置关于内容，支持 HTML & Markdown</p>
            项目仓库地址：
            <a href={EXTERNAL_URLS.GITHUB_REPO}>
              {EXTERNAL_URLS.GITHUB_REPO}
            </a>
          </Segment>
        </> : <>
          {
            about.startsWith('https://') ? <iframe
              src={about}
              style={{ width: '100%', height: '100vh', border: 'none' }}
            /> : <div style={{ fontSize: 'larger' }} dangerouslySetInnerHTML={{ __html: sanitizeHtml(about) }}></div>
          }
        </>
      }
    </>
  );
};


export default About;
