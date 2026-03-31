import { List } from "@refinedev/antd";
import { Alert, Button, InputNumber, Space, Table, Tag, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import axios from "axios";
import { useState } from "react";

import { consoleApiUrl } from "../../config";

interface Item {
  item_id: string;
  author_agent_id: string;
  raw_content: string;
  raw_notes: string;
  raw_url: string;
  status: number;
  summary: string | null;
  broadcast_type: string | null;
  domains: string[] | null;
  keywords: string[] | null;
  expire_time: string | null;
  geo: string | null;
  source_type: string | null;
  expected_response: string | null;
  group_id: string | null;
  created_at: number;
  updated_at: number;
}

interface ImprData {
  agent_id: string;
  item_ids: string[];
  group_ids: string[];
  urls: string[];
  items: Item[];
}

interface ImprResp {
  code: number;
  msg: string;
  data?: ImprData;
}

const formatTimestamp = (ts: number) => {
  if (!ts) return "-";
  return new Date(ts).toLocaleString();
};

const LongText = ({ text, maxWidth = 240 }: { text: string | null; maxWidth?: number }) => {
  if (!text) return <>-</>;
  return (
    <Typography.Paragraph
      copyable={{ tooltips: false }}
      ellipsis={{ rows: 5, expandable: true, symbol: "more" }}
      style={{ marginBottom: 0, maxWidth, whiteSpace: "pre-wrap" }}
    >
      {text}
    </Typography.Paragraph>
  );
};

export const ImprRecordList = () => {
  const [inputAgentID, setInputAgentID] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string>("");
  const [data, setData] = useState<ImprData | null>(null);

  const query = async () => {
    if (!inputAgentID || inputAgentID <= 0) {
      setErrorMsg("Please enter a valid agent_id");
      return;
    }

    setLoading(true);
    setErrorMsg("");

    try {
      const resp = await axios.get<ImprResp>(`${consoleApiUrl}/impr/items`, {
        params: { agent_id: inputAgentID },
      });

      if (resp.data.code !== 0 || !resp.data.data) {
        throw new Error(resp.data.msg || "query failed");
      }

      setData(resp.data.data);
    } catch (err) {
      setData(null);
      if (axios.isAxiosError(err) && err.response?.data?.msg) {
        setErrorMsg(String(err.response.data.msg));
      } else if (err instanceof Error) {
        setErrorMsg(err.message);
      } else {
        setErrorMsg("Query failed");
      }
    } finally {
      setLoading(false);
    }
  };

  const columns: ColumnsType<Item> = [
    {
      title: "Item ID",
      dataIndex: "item_id",
      key: "item_id",
      width: 100,
      fixed: "left",
    },
    {
      title: "Author",
      dataIndex: "author_agent_id",
      key: "author_agent_id",
      width: 100,
    },
    {
      title: "Raw Content",
      dataIndex: "raw_content",
      key: "raw_content",
      width: 260,
      render: (text: string) => <LongText text={text} maxWidth={260} />,
    },
    {
      title: "Summary",
      dataIndex: "summary",
      key: "summary",
      width: 240,
      render: (text: string | null) => <LongText text={text} maxWidth={240} />,
    },
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      width: 100,
      render: (status: number) => <Tag>{status}</Tag>,
    },
    {
      title: "Group ID",
      dataIndex: "group_id",
      key: "group_id",
      width: 120,
      render: (id: string | null) => id || "-",
    },
    {
      title: "Updated At",
      dataIndex: "updated_at",
      key: "updated_at",
      width: 180,
      render: (ts: number) => formatTimestamp(ts),
    },
  ];

  return (
    <List
      headerButtons={(
        <Space>
          <InputNumber
            min={1}
            precision={0}
            placeholder="Agent ID"
            value={inputAgentID}
            onChange={(value) => setInputAgentID(value)}
            style={{ width: 180 }}
          />
          <Button type="primary" loading={loading} onClick={query}>
            Search
          </Button>
        </Space>
      )}
      title="Impr Records"
    >
      {errorMsg && (
        <Alert
          type="error"
          message={errorMsg}
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {data && (
        <>
          <Space direction="vertical" style={{ width: "100%", marginBottom: 16 }}>
            <Typography.Text>Agent ID: {data.agent_id}</Typography.Text>
            <Typography.Text>
              item_ids: {data.item_ids.length > 0 ? data.item_ids.join(", ") : "-"}
            </Typography.Text>
            <Typography.Text>
              group_ids: {data.group_ids.length > 0 ? data.group_ids.join(", ") : "-"}
            </Typography.Text>
            <Typography.Text>
              urls: {data.urls.length > 0 ? data.urls.join(", ") : "-"}
            </Typography.Text>
          </Space>

          <Table
            dataSource={data.items}
            columns={columns}
            rowKey="item_id"
            loading={loading}
            scroll={{ x: 1200 }}
            pagination={{
              pageSize: 20,
              showSizeChanger: true,
              pageSizeOptions: [10, 20, 50, 100],
            }}
          />
        </>
      )}
    </List>
  );
};
