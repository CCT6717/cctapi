import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Dropdown,
  Form,
  Input,
  Message,
  Pagination,
  Popup,
  Table,
} from 'semantic-ui-react';
import { Link } from 'react-router-dom';
import { setPromptShown, shouldShowPrompt } from '../helpers';
import { ITEMS_PER_PAGE } from '../constants';
import { renderGroup } from '../helpers/render';
import { useChannelsTable } from './hooks/useChannelsTable';
import {
  renderTimestamp,
  renderType,
  renderBalance,
  renderStatus,
  renderResponseTime,
} from './utils/channelRenderers';

const promptID = 'detail';

const ChannelsTable = () => {
  const { t } = useTranslation();
  const {
    channels,
    loading,
    activePage,
    searchKeyword,
    searching,
    updatingBalance,
    setActivePage,
    refresh,
    onPaginationChange,
    manageChannel,
    searchChannels,
    switchTestModel,
    testChannel,
    testChannels,
    deleteAllDisabledChannels,
    updateChannelBalance,
    updateAllChannelsBalance,
    handleKeywordChange,
    sortChannel,
  } = useChannelsTable(t);

  const [showPrompt, setShowPrompt] = useState(shouldShowPrompt(promptID));
  const isShowDetail = () => localStorage.getItem('show_detail') === 'true';
  const [showDetail, setShowDetail] = useState(isShowDetail());
  const toggleShowDetail = () => {
    setShowDetail(!showDetail);
    localStorage.setItem('show_detail', (!showDetail).toString());
  };

  return (
    <>
      <Form onSubmit={searchChannels}>
        <Form.Input
          icon='search'
          fluid
          iconPosition='left'
          placeholder={t('channel.search')}
          value={searchKeyword}
          loading={searching}
          onChange={handleKeywordChange}
        />
      </Form>
      {showPrompt && (
        <Message
          onDismiss={() => {
            setShowPrompt(false);
            setPromptShown(promptID);
          }}
        >
          {t('channel.balance_notice')}
          <br />
          {t('channel.test_notice')}
          <br />
          {t('channel.detail_notice')}
        </Message>
      )}
      <Table basic={'very'} compact size='small'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('id');
              }}
            >
              {t('channel.table.id')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('name');
              }}
            >
              {t('channel.table.name')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('group');
              }}
            >
              {t('channel.table.group')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('type');
              }}
            >
              {t('channel.table.type')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('status');
              }}
            >
              {t('channel.table.status')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('response_time');
              }}
            >
              {t('channel.table.response_time')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('balance');
              }}
            >
              {t('channel.table.balance')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('priority');
              }}
              hidden={!showDetail}
            >
              {t('channel.table.priority')}
            </Table.HeaderCell>
            <Table.HeaderCell hidden={!showDetail}>
              {t('channel.table.test_model')}
            </Table.HeaderCell>
            <Table.HeaderCell>{t('channel.table.actions')}</Table.HeaderCell>
          </Table.Row>
        </Table.Header>

        <Table.Body>
          {channels
            .slice(
              (activePage - 1) * ITEMS_PER_PAGE,
              activePage * ITEMS_PER_PAGE
            )
            .map((channel, idx) => {
              if (channel.deleted) return <></>;
              return (
                <Table.Row key={channel.id}>
                  <Table.Cell>{channel.id}</Table.Cell>
                  <Table.Cell>
                    {channel.name ? channel.name : t('channel.table.no_name')}
                  </Table.Cell>
                  <Table.Cell>{renderGroup(channel.group)}</Table.Cell>
                  <Table.Cell>{renderType(channel.type, t)}</Table.Cell>
                  <Table.Cell>{renderStatus(channel.status, t)}</Table.Cell>
                  <Table.Cell>
                    <Popup
                      content={
                        channel.test_time
                          ? renderTimestamp(channel.test_time)
                          : t('channel.table.not_tested')
                      }
                      key={channel.id}
                      trigger={renderResponseTime(channel.response_time, t)}
                      basic
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Popup
                      trigger={
                        <span
                          onClick={() => {
                            updateChannelBalance(channel.id, channel.name, idx);
                          }}
                          style={{ cursor: 'pointer' }}
                        >
                          {renderBalance(channel.type, channel.balance, t)}
                        </span>
                      }
                      content={t('channel.table.click_to_update')}
                      basic
                    />
                  </Table.Cell>
                  <Table.Cell hidden={!showDetail}>
                    <Popup
                      trigger={
                        <Input
                          type='number'
                          defaultValue={channel.priority}
                          onBlur={(event) => {
                            manageChannel(
                              channel.id,
                              'priority',
                              idx,
                              event.target.value
                            );
                          }}
                        >
                          <input style={{ maxWidth: '60px' }} />
                        </Input>
                      }
                      content={t('channel.table.priority_tip')}
                      basic
                    />
                  </Table.Cell>
                  <Table.Cell hidden={!showDetail}>
                    <Dropdown
                      placeholder={t('channel.table.select_test_model')}
                      selection
                      options={channel.model_options}
                      defaultValue={channel.test_model}
                      onChange={(event, data) => {
                        switchTestModel(idx, data.value);
                      }}
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        flexWrap: 'wrap',
                        gap: '2px',
                        rowGap: '6px',
                      }}
                    >
                      <Button
                        size={'tiny'}
                        positive
                        onClick={() => {
                          testChannel(
                            channel.id,
                            channel.name,
                            idx,
                            channel.test_model
                          );
                        }}
                      >
                        {t('channel.buttons.test')}
                      </Button>
                      <Popup
                        trigger={
                          <Button size='tiny' negative>
                            {t('channel.buttons.delete')}
                          </Button>
                        }
                        on='click'
                        flowing
                        hoverable
                      >
                        <Button
                          size={'tiny'}
                          negative
                          onClick={() => {
                            manageChannel(channel.id, 'delete', idx);
                          }}
                        >
                          {t('channel.buttons.confirm_delete')} {channel.name}
                        </Button>
                      </Popup>
                      <Button
                        size={'tiny'}
                        onClick={() => {
                          manageChannel(
                            channel.id,
                            channel.status === 1 ? 'disable' : 'enable',
                            idx
                          );
                        }}
                      >
                        {channel.status === 1
                          ? t('channel.buttons.disable')
                          : t('channel.buttons.enable')}
                      </Button>
                      <Button
                        size={'tiny'}
                        as={Link}
                        to={'/channel/edit/' + channel.id}
                      >
                        {t('channel.buttons.edit')}
                      </Button>
                    </div>
                  </Table.Cell>
                </Table.Row>
              );
            })}
        </Table.Body>

        <Table.Footer>
          <Table.Row>
            <Table.HeaderCell colSpan={showDetail ? '10' : '8'}>
              <Button size='tiny' as={Link} to='/channel/add' loading={loading}>
                {t('channel.buttons.add')}
              </Button>
              <Button
                size='tiny'
                loading={loading}
                onClick={() => {
                  testChannels('all');
                }}
              >
                {t('channel.buttons.test_all')}
              </Button>
              <Button
                size='tiny'
                loading={loading}
                onClick={() => {
                  testChannels('disabled');
                }}
              >
                {t('channel.buttons.test_disabled')}
              </Button>
              <Popup
                trigger={
                  <Button size='tiny' loading={loading}>
                    {t('channel.buttons.delete_disabled')}
                  </Button>
                }
                on='click'
                flowing
                hoverable
              >
                <Button
                  size='tiny'
                  loading={loading}
                  negative
                  onClick={deleteAllDisabledChannels}
                >
                  {t('channel.buttons.confirm_delete_disabled')}
                </Button>
              </Popup>
              <Pagination
                floated='right'
                activePage={activePage}
                onPageChange={onPaginationChange}
                size='tiny'
                siblingRange={1}
                totalPages={
                  Math.ceil(channels.length / ITEMS_PER_PAGE) +
                  (channels.length % ITEMS_PER_PAGE === 0 ? 1 : 0)
                }
              />
              <Button size='tiny' onClick={refresh} loading={loading}>
                {t('channel.buttons.refresh')}
              </Button>
              <Button size='tiny' onClick={toggleShowDetail}>
                {showDetail
                  ? t('channel.buttons.hide_detail')
                  : t('channel.buttons.show_detail')}
              </Button>
            </Table.HeaderCell>
          </Table.Row>
        </Table.Footer>
      </Table>
    </>
  );
};

export default ChannelsTable;
